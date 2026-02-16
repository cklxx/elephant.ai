#!/usr/bin/env python3
"""drive-file skill — 飞书云盘文件管理。

通过 channel tool 的 drive actions 管理飞书云盘文件和文件夹。
当前为框架实现，实际调用通过 channel tool 的 list_drive_*/create_drive_*/copy_drive_*/delete_drive_* actions。
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
        return {"error": "LARK_TENANT_TOKEN not set, drive operations unavailable"}

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


def list_files(args: dict) -> dict:
    folder_token = args.get("folder_token", "root")

    params = f"?folder_token={folder_token}"
    if args.get("page_size"):
        params += f"&page_size={args['page_size']}"
    if args.get("page_token"):
        params += f"&page_token={args['page_token']}"

    result = _lark_api("GET", f"/drive/v1/files{params}")
    if "error" in result:
        return {"success": False, **result}
    files = result.get("data", {}).get("files", [])
    return {"success": True, "files": files, "count": len(files)}


def create_folder(args: dict) -> dict:
    name = args.get("name", "")
    if not name:
        return {"success": False, "error": "name is required"}

    folder_token = args.get("folder_token", "root")
    body = {"name": name, "folder_token": folder_token}

    result = _lark_api("POST", "/drive/v1/files/create_folder", body)
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "folder": result.get("data", {}), "message": f"文件夹「{name}」已创建"}


def copy_file(args: dict) -> dict:
    file_token = args.get("file_token", "")
    folder_token = args.get("folder_token", "")
    name = args.get("name", "")
    if not all([file_token, folder_token, name]):
        return {"success": False, "error": "file_token, folder_token, and name are required"}

    body = {
        "name": name,
        "folder_token": folder_token,
        "type": args.get("file_type", "file"),
    }

    result = _lark_api("POST", f"/drive/v1/files/{file_token}/copy", body)
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "file": result.get("data", {}).get("file", {}), "message": "文件已复制"}


def delete_file(args: dict) -> dict:
    file_token = args.get("file_token", "")
    if not file_token:
        return {"success": False, "error": "file_token is required"}

    file_type = args.get("file_type", "file")

    result = _lark_api("DELETE", f"/drive/v1/files/{file_token}?type={file_type}")
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "message": f"文件 {file_token} 已删除"}


def run(args: dict) -> dict:
    action = args.pop("action", "list_files")

    handlers = {
        "list_files": list_files,
        "create_folder": create_folder,
        "copy_file": copy_file,
        "delete_file": delete_file,
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
