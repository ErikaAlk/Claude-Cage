from contextlib import closing
import fcntl
import json
import os
from pathlib import Path
import selectors
import subprocess
import sys
import time
from urllib.parse import urlsplit

from zhigeng.hooks import build_notification, enqueue, flush_source, queue_connection

ROOT = Path.home() / ".local/share/cage/qq-notify"
QUEUE = ROOT / "queue.sqlite3"
SOURCE = "cage"


def capture():
    event = json.load(sys.stdin)
    if event.get("hook_event_name") != "Stop":
        return
    notification = build_notification("claude", event)
    if notification:
        notification["title"] = ("Cage · " + (Path(event.get("cwd", "")).name or "Claude Code"))[:200]
        enqueue({"queue_file": str(QUEUE)}, SOURCE, notification)


def deliver():
    url = os.environ["ROBIN_URL"]
    if urlsplit(url).scheme != "https":
        raise ValueError("Robin 服务地址必须使用 HTTPS")
    fd = os.memfd_create("robin-notify")
    with os.fdopen(fd, "wb") as token:
        token.write(os.environ.pop("ROBIN_TOKEN").encode())
        token.flush()
        config = {"base_url": url, "queue_file": str(QUEUE), "tokens": {SOURCE: f"/proc/self/fd/{fd}"}}
        flush_source(config, SOURCE)


def queue_status(resume=False):
    if not QUEUE.exists():
        print("队列为空")
        return
    with closing(queue_connection(QUEUE)) as db:
        if resume:
            with db:
                db.execute("DELETE FROM queue_sources WHERE source=? AND status='blocked'", (SOURCE,))
        rows = db.execute("SELECT status,count(*) FROM queue WHERE source=? GROUP BY status", (SOURCE,))
        blocked = db.execute("SELECT code FROM queue_sources WHERE source=? AND status='blocked'", (SOURCE,)).fetchone()
        print(json.dumps({"投递状态": dict(rows), "暂停发送的 HTTP 状态码": blocked[0] if blocked else None}, ensure_ascii=False))


def respond(line):
    request = json.loads(line)
    if "id" not in request:
        return
    reply = {"jsonrpc": "2.0", "id": request["id"]}
    method = request["method"]
    if method == "initialize":
        result = {"protocolVersion": request["params"]["protocolVersion"], "capabilities": {"tools": {}},
                  "serverInfo": {"name": "qq-notify", "version": "1.0.0"}}
    elif method == "ping":
        result = {}
    elif method == "tools/list":
        result = {"tools": []}
    else:
        reply["error"] = {"code": -32601, "message": "不支持此方法"}
    if "error" not in reply:
        reply["result"] = result
    print(json.dumps(reply), flush=True)


def serve():
    with (ROOT / "active.lock").open("a") as lock, selectors.DefaultSelector() as selector:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        selector.register(sys.stdin, selectors.EVENT_READ)
        worker = None
        next_check = 0
        try:
            while True:
                if selector.select(1):
                    line = sys.stdin.buffer.raw.readline()
                    if not line:
                        return
                    respond(line)
                if worker is not None and worker.poll() is not None:
                    if worker.returncode:
                        print("QQ 投递未完成，请检查网络及通知队列状态。", file=sys.stderr, flush=True)
                    next_check = time.monotonic() + (60 if worker.returncode else 0)
                    worker = None
                if worker is None and time.monotonic() >= next_check and QUEUE.exists():
                    with closing(queue_connection(QUEUE)) as db:
                        ready = db.execute("SELECT 1 FROM sqlite_master WHERE name='queue'").fetchone()
                        blocked = db.execute("SELECT 1 FROM queue_sources WHERE source=? AND status='blocked'", (SOURCE,)).fetchone()
                        pending = ready and not blocked and db.execute("SELECT 1 FROM queue WHERE source=? AND status='pending' LIMIT 1", (SOURCE,)).fetchone()
                    next_check = time.monotonic() + 2
                    if pending:
                        worker = subprocess.Popen([sys.executable, "-I", __file__, "deliver"], stdin=subprocess.DEVNULL,
                                                  stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        finally:
            if worker is not None:
                worker.terminate()
                worker.wait()


if __name__ == "__main__":
    os.umask(0o077)
    {"enqueue": capture, "deliver": deliver, "serve": serve, "status": queue_status,
     "resume": lambda: queue_status(resume=True)}[sys.argv[1]]()
