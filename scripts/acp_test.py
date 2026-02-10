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


def resolve_default_addr():
    env_addr = os.getenv("ACP_ADDR") or os.getenv("ACP_SERVER_ADDR")
    if env_addr:
        return env_addr

    host = os.getenv("ACP_HOST") or os.getenv("ACP_SERVER_HOST") or "127.0.0.1"
    port_file = os.getenv("ACP_PORT_FILE")
    if not port_file:
        config_path = os.getenv("ALEX_CONFIG_PATH") or os.path.join(os.path.expanduser("~"), ".alex", "config.yaml")
        config_path = os.path.abspath(os.path.expanduser(config_path))
        port_file = os.path.join(os.path.dirname(config_path), "pids", "acp.port")

    port = ""
    try:
        with open(port_file, "r", encoding="utf-8") as handle:
            port = handle.read().strip()
    except FileNotFoundError:
        port = ""

    if port:
        return f"http://{host}:{port}"

    return "http://127.0.0.1:9000"


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


def resolve_default_cwds():
    env_cwd = os.getenv("ACP_CWD")
    if env_cwd:
        return [os.path.abspath(env_cwd)]
    return ["/workspace", os.getcwd()]


def main():
    parser = argparse.ArgumentParser(description="ACP smoke test client (HTTP/SSE JSON-RPC)")
    parser.add_argument(
        "--addr",
        default="",
        help="ACP server base URL (defaults to ACP_ADDR/ACP_SERVER_ADDR or <config_dir>/pids/acp.port)",
    )
    parser.add_argument("--cwd", default=None, help="Session working directory")
    parser.add_argument("--prompt", default="Hello ACP", help="Prompt text")
    args = parser.parse_args()

    addr = args.addr.strip() if args.addr else ""
    if not addr:
        addr = resolve_default_addr()
    base_url = normalize_base_url(addr)
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

    request_id = 1
    post_json(rpc_url, {"jsonrpc": "2.0", "id": request_id, "method": "initialize", "params": {"protocolVersion": 1}})
    print("<<", read_response(out_queue, request_id))

    request_id += 1
    session_id = None
    cwd_candidates = [os.path.abspath(args.cwd)] if args.cwd else resolve_default_cwds()
    for cwd in cwd_candidates:
        post_json(
            rpc_url,
            {
                "jsonrpc": "2.0",
                "id": request_id,
                "method": "session/new",
                "params": {"cwd": cwd, "mcpServers": []},
            },
        )
        resp = read_response(out_queue, request_id)
        print("<<", resp)
        if "error" in resp:
            request_id += 1
            continue
        session_id = resp.get("result", {}).get("sessionId")
        if session_id:
            break
        request_id += 1

    if not session_id:
        print("sessionId missing")
        return 2

    request_id += 1
    post_json(
        rpc_url,
        {
            "jsonrpc": "2.0",
            "id": request_id,
            "method": "session/prompt",
            "params": {"sessionId": session_id, "prompt": [{"type": "text", "text": args.prompt}]},
        },
    )

    while True:
        msg = out_queue.get()
        print("<<", msg)
        if msg.get("id") == request_id:
            break

    stop_event.set()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
