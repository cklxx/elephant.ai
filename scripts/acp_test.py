#!/usr/bin/env python3
import argparse
import json
import os
import queue
import threading
import time
import urllib.parse
import urllib.request
import uuid


def normalize_base_url(raw):
    raw = raw.strip()
    if not raw:
        raise SystemExit("addr is required")
    if "://" not in raw:
        raw = "http://" + raw
    return raw.rstrip("/")


def post_json(url, payload):
    data = json.dumps(payload, separators=(",", ":")).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=data,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req, timeout=10) as resp:
        resp.read()


def sse_reader(url, out_queue, ready_event, stop_event):
    req = urllib.request.Request(url, headers={"Accept": "text/event-stream"})
    with urllib.request.urlopen(req, timeout=10) as resp:
        ready_event.set()
        data_lines = []
        while not stop_event.is_set():
            line = resp.readline()
            if not line:
                break
            text = line.decode("utf-8").rstrip("\r\n")
            if not text:
                if data_lines:
                    payload = "\n".join(data_lines).strip()
                    data_lines = []
                    if payload:
                        try:
                            out_queue.put(json.loads(payload))
                        except json.JSONDecodeError:
                            out_queue.put({"raw": payload})
                continue
            if text.startswith(":"):
                continue
            if text.startswith("data:"):
                data_lines.append(text[len("data:") :].strip())


def read_response(out_queue, want_id, timeout=10):
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            msg = out_queue.get(timeout=0.1)
        except queue.Empty:
            continue
        if msg.get("id") == want_id:
            return msg
        print("<<", msg)
    raise SystemExit(f"timeout waiting for response id={want_id}")


def main():
    parser = argparse.ArgumentParser(description="ACP smoke test client (HTTP/SSE JSON-RPC)")
    parser.add_argument("--addr", default="http://127.0.0.1:9000", help="ACP server base URL")
    parser.add_argument("--cwd", default=os.getcwd(), help="Session working directory")
    parser.add_argument("--prompt", default="Hello ACP", help="Prompt text")
    args = parser.parse_args()

    if not os.path.isabs(args.cwd):
        print("cwd must be absolute")
        return 2

    base_url = normalize_base_url(args.addr)
    client_id = uuid.uuid4().hex
    sse_url = f"{base_url}/acp/sse?client_id={urllib.parse.quote(client_id)}"
    rpc_url = f"{base_url}/acp/rpc?client_id={urllib.parse.quote(client_id)}"

    out_queue = queue.Queue()
    ready_event = threading.Event()
    stop_event = threading.Event()

    thread = threading.Thread(
        target=sse_reader,
        args=(sse_url, out_queue, ready_event, stop_event),
        daemon=True,
    )
    thread.start()

    if not ready_event.wait(timeout=5):
        raise SystemExit("failed to establish SSE connection")

    post_json(rpc_url, {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": 1}})
    print("<<", read_response(out_queue, 1))

    post_json(
        rpc_url,
        {
            "jsonrpc": "2.0",
            "id": 2,
            "method": "session/new",
            "params": {"cwd": args.cwd, "mcpServers": []},
        },
    )
    resp = read_response(out_queue, 2)
    print("<<", resp)
    session_id = resp.get("result", {}).get("sessionId")
    if not session_id:
        print("sessionId missing")
        return 2

    post_json(
        rpc_url,
        {
            "jsonrpc": "2.0",
            "id": 3,
            "method": "session/prompt",
            "params": {"sessionId": session_id, "prompt": [{"type": "text", "text": args.prompt}]},
        },
    )

    while True:
        msg = out_queue.get()
        print("<<", msg)
        if msg.get("id") == 3:
            break

    stop_event.set()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
