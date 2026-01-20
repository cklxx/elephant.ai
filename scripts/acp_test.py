#!/usr/bin/env python3
import argparse
import json
import os
import socket
import sys


def send(sock, obj):
    data = json.dumps(obj, separators=(",", ":")).encode("utf-8") + b"\n"
    sock.sendall(data)


def read_message(sock_file):
    line = sock_file.readline()
    if not line:
        raise SystemExit("connection closed")
    if line.lower().startswith(b"content-length:"):
        length = int(line.split(b":", 1)[1].strip())
        while True:
            header = sock_file.readline()
            if not header or header == b"\r\n":
                break
        payload = sock_file.read(length)
        if not payload:
            raise SystemExit("connection closed")
        return payload.decode("utf-8")
    return line.decode("utf-8")


def main():
    parser = argparse.ArgumentParser(description="ACP smoke test client (TCP JSON-RPC)")
    parser.add_argument("--host", default="127.0.0.1", help="ACP server host")
    parser.add_argument("--port", type=int, default=9000, help="ACP server port")
    parser.add_argument("--cwd", default=os.getcwd(), help="Session working directory")
    parser.add_argument("--prompt", default="Hello ACP", help="Prompt text")
    args = parser.parse_args()

    if not os.path.isabs(args.cwd):
        print("cwd must be absolute", file=sys.stderr)
        return 2

    sock = socket.create_connection((args.host, args.port))
    sock_file = sock.makefile("rb")

    send(sock, {"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": {"protocolVersion": 1}})
    print("<<", read_message(sock_file).strip())

    send(sock, {"jsonrpc": "2.0", "id": 2, "method": "session/new", "params": {"cwd": args.cwd, "mcpServers": []}})
    resp = json.loads(read_message(sock_file))
    print("<<", resp)
    session_id = resp.get("result", {}).get("sessionId")
    if not session_id:
        print("sessionId missing", file=sys.stderr)
        return 2

    send(
        sock,
        {
            "jsonrpc": "2.0",
            "id": 3,
            "method": "session/prompt",
            "params": {"sessionId": session_id, "prompt": [{"type": "text", "text": args.prompt}]},
        },
    )

    while True:
        msg = json.loads(read_message(sock_file))
        print("<<", msg)
        if msg.get("id") == 3:
            break

    sock.close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
