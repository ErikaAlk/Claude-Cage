import json
import selectors
import socket
import sys


with socket.socket(socket.AF_UNIX, socket.SOCK_STREAM) as service, selectors.DefaultSelector() as events:
    service.settimeout(10)
    service.connect("/run/cage-popup-layout.sock")
    if service.makefile("rb").readline() != b"ready\n":
        raise RuntimeError("弹窗布局服务启动失败")
    events.register(service, selectors.EVENT_READ)
    events.register(sys.stdin, selectors.EVENT_READ)
    while True:
        for event, _ in events.select():
            if event.fileobj is service:
                raise RuntimeError("弹窗布局服务已断开")
            line = sys.stdin.buffer.raw.readline()
            if not line:
                sys.exit(0)
            message = json.loads(line)
            if "id" not in message:
                continue
            reply = {"jsonrpc": "2.0", "id": message["id"]}
            method = message["method"]
            if method == "initialize":
                reply["result"] = {"protocolVersion": message["params"]["protocolVersion"],
                                   "capabilities": {"tools": {}}, "serverInfo": {"name": "layout", "version": "1.0.0"}}
            elif method == "tools/list":
                reply["result"] = {"tools": []}
            elif method == "ping":
                reply["result"] = {}
            else:
                reply["error"] = {"code": -32601, "message": "不支持此方法"}
            print(json.dumps(reply), flush=True)
