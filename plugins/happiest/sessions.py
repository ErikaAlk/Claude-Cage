import base64
from collections import OrderedDict
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys

fingerprints = OrderedDict()


def history_fingerprint(stream, info, size, session, config):
    key = (str(config), session, info.st_dev, info.st_ino, info.st_ctime_ns, info.st_mtime_ns, size)
    if key in fingerprints:
        fingerprints.move_to_end(key)
        return fingerprints[key]
    digest = hashlib.sha256(json.dumps(list(key[:4])).encode())
    stream.seek(0)
    remaining = size
    while remaining:
        block = stream.read(min(remaining, 1 << 20))
        if not block:
            return ""
        digest.update(block)
        remaining -= len(block)
    result = digest.hexdigest()
    fingerprints[key] = result
    if len(fingerprints) > 128:
        fingerprints.popitem(last=False)
    return result

def config_path(request):
    if request.get("config"):
        return Path(request["config"]).resolve()
    account = request.get("account") or "default"
    if account == "default":
        return Path.home() / ".claude"
    if not re.fullmatch(r"[0-9a-f]{24}", account):
        raise ValueError("账号标识无效")
    return Path.home() / ".local/share/cage/claude-accounts" / account / "config"


def read_history(request, session, config):
    offset, maximum = request["offset"], request["max_bytes"]
    if type(offset) is not int or offset < 0 or type(maximum) is not int or not 0 < maximum <= 3 << 20:
        return {"error": "invalid_transcript_request"}
    source = request.get("source", "")
    if offset and not source:
        return {"error": "source_required"}
    projects = config / "projects"
    matches = list(projects.glob("*/" + session + ".jsonl"))
    if not matches:
        if source or request.get("required"):
            return {"error": "source_changed" if source else "transcript_unavailable"}
        return {"cli_id": session, "content": "", "offset": 0, "next_offset": 0,
                "total_bytes": 0, "eof": True, "source": ""}
    if len(matches) != 1:
        return {"error": "transcript_ambiguous"}
    path = matches[0]
    if projects.is_symlink() or path.parent.is_symlink() or path.is_symlink():
        return {"error": "source_changed" if source else "transcript_unavailable"}
    with open(path, "rb", opener=lambda name, flags: os.open(name, flags | os.O_NOFOLLOW | os.O_NONBLOCK)) as stream:
        info = os.fstat(stream.fileno())
        if not stat.S_ISREG(info.st_mode):
            return {"error": "transcript_unavailable"}
        if source:
            match = re.fullmatch(r"(0|[1-9][0-9]*)-([0-9a-f]{64})", source)
            if not match or info.st_size < int(match[1]) or match[2] != history_fingerprint(stream, info, int(match[1]), session, config):
                return {"error": "source_changed"}
        if offset > info.st_size:
            return {"error": "source_changed"}
        identity = history_fingerprint(stream, info, info.st_size, session, config)
        if not identity:
            return {"error": "source_changed"}
        if info.st_size:
            workspace = request["workspace"].rstrip("/")
            stream.seek(0)
            for line in stream:
                row = json.loads(line)
                directory = row.get("cwd")
                if directory:
                    if directory != workspace and not directory.startswith(workspace + "/"):
                        return {"error": "transcript_workspace_mismatch"}
                    break
            else:
                return {"error": "transcript_workspace_mismatch"}
        stream.seek(offset)
        data = stream.read(min(maximum, info.st_size - offset))
        current = path.stat(follow_symlinks=False)
        if not os.path.samestat(info, current) or current.st_ctime_ns != info.st_ctime_ns or len(data) != min(maximum, info.st_size - offset):
            return {"error": "source_changed"}
    following = offset + len(data)
    return {"cli_id": session, "content": base64.b64encode(data).decode("ascii"), "offset": offset,
            "next_offset": following, "total_bytes": info.st_size, "eof": following == info.st_size,
            "source": str(info.st_size) + "-" + identity}


def session_status(session, config):
    listing = subprocess.run(["claude", "agents", "--json"], env={**os.environ, "CLAUDE_CONFIG_DIR": str(config)},
                             capture_output=True, text=True, check=True, timeout=10)
    running = any(row.get("sessionId") == session and type(row.get("pid")) is int and
                  Path("/proc", str(row["pid"])).is_dir() for row in json.loads(listing.stdout))
    return {"cli_id": session, "running": running}


def tool(name, description):
    return {"name": name, "description": description, "inputSchema": {"type": "object"},
            "_meta": {"ui": {"visibility": ["app"]}}}


def answer(message):
    method = message["method"]
    params = message.get("params", {})
    if method == "initialize":
        return {"protocolVersion": params["protocolVersion"], "capabilities": {"tools": {}},
                "serverInfo": {"name": "sessions", "version": "1.0.0"}}
    if method == "ping":
        return {}
    if method == "tools/list":
        return {"tools": [tool("read_session_history", "读取选定会话的一页原始记录"),
                          tool("check_session_process", "检查选定会话的进程是否仍在运行")]}
    if method != "tools/call":
        return None
    request = params["arguments"]
    session = request["cli_id"]
    if not isinstance(session, str) or not re.fullmatch(r"[a-f0-9]{8}-(?:[a-f0-9]{4}-){3}[a-f0-9]{12}", session):
        return {"content": [], "structuredContent": {"error": "invalid_transcript_request"}}
    config = config_path(request)
    if params["name"] == "read_session_history":
        result = read_history(request, session, config)
    elif params["name"] == "check_session_process":
        result = session_status(session, config)
    else:
        return None
    return {"content": [], "structuredContent": result}


if __name__ == "__main__":
    for line in sys.stdin:
        message = json.loads(line)
        if "id" in message:
            result = answer(message)
            reply = {"jsonrpc": "2.0", "id": message["id"]}
            if result is None:
                reply["error"] = {"code": -32601, "message": "不支持此方法"}
            else:
                reply["result"] = result
            print(json.dumps(reply), flush=True)
