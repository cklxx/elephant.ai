#!/usr/bin/env python3
"""drive-file skill — 飞书云盘文件管理。"""

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

from skill_runner.lark_auth import lark_api_json


def _lark_api(
    method: str,
    path: str,
    body: dict | None = None,
    *,
    query: dict | str | None = None,
) -> dict:
    return lark_api_json(method, path, body, query=query)


def _api_failure(result: dict) -> dict | None:
    if "error" in result:
        return {"success": False, **result}
    code = result.get("code", 0)
    if isinstance(code, int) and code != 0:
        return {"success": False, "code": code, "error": result.get("msg") or f"Lark API error {code}"}
    return None


def _resolve_default_folder_token() -> str:
    for key in ("LARK_DRIVE_FOLDER_TOKEN", "LARK_DRIVE_DEFAULT_FOLDER_TOKEN", "LARK_DRIVE_FOLDER_ID"):
        value = os.environ.get(key, "").strip()
        if value:
            return value
    return ""


def _resolve_root_folder_token() -> str:
    result = _lark_api("GET", "/drive/explorer/v2/root_folder/meta")
    if _api_failure(result):
        return ""
    data = result.get("data", {})
    return str(
        data.get("token")
        or data.get("root_folder_token")
        or data.get("folder_token")
        or ""
    ).strip()


def _resolve_folder_token(args: dict, *, allow_empty: bool = False, resolve_root: bool = False) -> str:
    provided = str(args.get("folder_token", "")).strip()
    if provided:
        return provided
    env_default = _resolve_default_folder_token()
    if env_default:
        return env_default
    if resolve_root:
        root_token = _resolve_root_folder_token()
        if root_token:
            return root_token
    return "" if allow_empty else ""


def list_files(args: dict) -> dict:
    folder_token = _resolve_folder_token(args, allow_empty=True)
    query: dict[str, str | int] = {"folder_token": folder_token}
    if args.get("page_size"):
        query["page_size"] = args["page_size"]
    if args.get("page_token"):
        query["page_token"] = args["page_token"]

    result = _lark_api("GET", "/drive/v1/files", query=query)
    failure = _api_failure(result)
    if failure:
        return failure
    files = result.get("data", {}).get("files", [])
    return {"success": True, "files": files, "count": len(files), "folder_token_used": folder_token}


def create_folder(args: dict) -> dict:
    name = args.get("name", "")
    if not name:
        return {"success": False, "error": "name is required"}

    folder_token = _resolve_folder_token(args, resolve_root=True)
    body = {"name": name}
    if folder_token:
        body["folder_token"] = folder_token

    result = _lark_api("POST", "/drive/v1/files/create_folder", body)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "folder": result.get("data", {}), "message": f"文件夹「{name}」已创建"}


def copy_file(args: dict) -> dict:
    file_token = args.get("file_token", "")
    name = args.get("name", "")
    if not file_token or not name:
        return {"success": False, "error": "file_token and name are required"}
    folder_token = _resolve_folder_token(args, resolve_root=True)
    if not folder_token:
        return {"success": False, "error": "folder_token is required"}

    body = {
        "name": name,
        "folder_token": folder_token,
        "type": args.get("file_type", "file"),
    }

    result = _lark_api("POST", f"/drive/v1/files/{file_token}/copy", body)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "file": result.get("data", {}).get("file", {}), "message": "文件已复制"}


def delete_file(args: dict) -> dict:
    file_token = args.get("file_token", "")
    if not file_token:
        return {"success": False, "error": "file_token is required"}

    file_type = args.get("file_type", "file")

    result = _lark_api("DELETE", f"/drive/v1/files/{file_token}", query={"type": file_type})
    failure = _api_failure(result)
    if failure:
        return failure
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
