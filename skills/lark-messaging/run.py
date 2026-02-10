#!/usr/bin/env python3
"""lark-messaging skill — Lark 消息发送。

通过 Lark Open API 发送消息到群组或个人。
"""

from __future__ import annotations

from pathlib import Path
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv

load_repo_dotenv(__file__)

import json
import os
import sys
import urllib.error
import urllib.request


_LARK_HOST = os.environ.get("LARK_HOST", "https://open.feishu.cn")


def _get_token() -> str:
    app_id = os.environ.get("LARK_APP_ID", "")
    app_secret = os.environ.get("LARK_APP_SECRET", "")
    if not app_id or not app_secret:
        return ""
    body = json.dumps({"app_id": app_id, "app_secret": app_secret}).encode()
    req = urllib.request.Request(
        f"{_LARK_HOST}/open-apis/auth/v3/tenant_access_token/internal",
        data=body,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            data = json.loads(resp.read().decode())
            return data.get("tenant_access_token", "")
    except urllib.error.URLError:
        return ""


def _lark_api(method: str, path: str, body: dict | None = None, token: str = "") -> dict:
    if not token:
        token = _get_token()
    if not token:
        return {"error": "LARK_APP_ID/LARK_APP_SECRET not set"}

    url = f"{_LARK_HOST}/open-apis{path}"
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(
        url, data=data,
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"},
        method=method,
    )
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.URLError as exc:
        return {"error": str(exc)}


def send(args: dict) -> dict:
    chat_id = args.get("chat_id", "")
    content = args.get("content", "")
    if not chat_id or not content:
        return {"success": False, "error": "chat_id and content are required"}

    msg_type = args.get("msg_type", "text")
    if msg_type == "text":
        body = {"receive_id": chat_id, "msg_type": "text", "content": json.dumps({"text": content})}
    else:
        body = {"receive_id": chat_id, "msg_type": msg_type, "content": content}

    result = _lark_api("POST", "/im/v1/messages?receive_id_type=chat_id", body)
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "message_id": result.get("data", {}).get("message_id", ""), "message": "消息已发送"}


def run(args: dict) -> dict:
    action = args.pop("action", "send")
    if action == "send":
        return send(args)
    return {"success": False, "error": f"unknown action: {action}"}


def main() -> None:
    if len(sys.argv) > 1:
        args = json.loads(sys.argv[1])
    elif not sys.stdin.isatty():
        args = json.load(sys.stdin)
    else:
        args = {}
    result = run(args)
    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    sys.exit(0 if result.get("success") else 1)


if __name__ == "__main__":
    main()
