#!/usr/bin/env python3
"""doc-management skill — 飞书云文档管理。

通过 channel tool 的 docx actions 管理飞书云文档。
当前为框架实现，实际调用通过 channel tool 的 create_doc/read_doc/read_doc_content actions。
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
import urllib.error
import urllib.request


def _lark_api(method: str, path: str, body: dict | None = None) -> dict:
    """Call Lark Open API (placeholder — needs tenant_access_token)."""
    base = "https://open.feishu.cn/open-apis"
    token = os.environ.get("LARK_TENANT_TOKEN", "")
    if not token:
        return {"error": "LARK_TENANT_TOKEN not set, docx operations unavailable"}

    url = f"{base}{path}"
    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json",
    }
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(url, data=data, headers=headers, method=method)

    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.URLError as exc:
        return {"error": str(exc)}


def create_doc(args: dict) -> dict:
    title = args.get("title", "")
    folder_token = args.get("folder_token", "")

    body = {"title": title}
    if folder_token:
        body["folder_token"] = folder_token

    result = _lark_api("POST", "/docx/v1/documents", body)
    if "error" in result:
        return {"success": False, **result}
    doc = result.get("data", {}).get("document", {})
    return {"success": True, "document": doc, "message": f"文档「{title}」已创建"}


def read_doc(args: dict) -> dict:
    document_id = args.get("document_id", "")
    if not document_id:
        return {"success": False, "error": "document_id is required"}

    result = _lark_api("GET", f"/docx/v1/documents/{document_id}")
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "document": result.get("data", {}).get("document", {})}


def read_doc_content(args: dict) -> dict:
    document_id = args.get("document_id", "")
    if not document_id:
        return {"success": False, "error": "document_id is required"}

    result = _lark_api("GET", f"/docx/v1/documents/{document_id}/raw_content")
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "content": result.get("data", {}).get("content", "")}


def run(args: dict) -> dict:
    action = args.pop("action", "read")

    handlers = {
        "create": create_doc,
        "read": read_doc,
        "read_content": read_doc_content,
    }
    handler = handlers.get(action)
    if not handler:
        return {"success": False, "error": f"unknown action: {action}, valid: {list(handlers)}"}
    return handler(args)


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
    sys.exit(0 if result.get("success", False) else 1)


if __name__ == "__main__":
    main()
