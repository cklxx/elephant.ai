#!/usr/bin/env python3
"""contact-lookup skill — 飞书通讯录查询。

通过 channel tool 的 contact actions 查询飞书通讯录。
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


def _lark_api(method: str, path: str) -> dict:
    base = "https://open.feishu.cn/open-apis"
    token = os.environ.get("LARK_TENANT_TOKEN", "")
    if not token:
        return {"error": "LARK_TENANT_TOKEN not set"}

    url = f"{base}{path}"
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
    req = urllib.request.Request(url, headers=headers, method=method)

    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.URLError as exc:
        return {"error": str(exc)}


def get_user(args: dict) -> dict:
    user_id = args.get("user_id", "")
    if not user_id:
        return {"success": False, "error": "user_id is required"}
    user_id_type = args.get("user_id_type", "open_id")
    result = _lark_api("GET", f"/contact/v3/users/{user_id}?user_id_type={user_id_type}")
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "user": result.get("data", {}).get("user", {})}


def list_users(args: dict) -> dict:
    dept_id = args.get("department_id", "")
    if not dept_id:
        return {"success": False, "error": "department_id is required"}
    page_size = args.get("page_size", 50)
    page_token = args.get("page_token", "")
    params = f"?department_id={dept_id}&page_size={page_size}"
    if page_token:
        params += f"&page_token={page_token}"
    result = _lark_api("GET", f"/contact/v3/users{params}")
    if "error" in result:
        return {"success": False, **result}
    data = result.get("data", {})
    return {"success": True, "users": data.get("items", []),
            "has_more": data.get("has_more", False)}


def get_department(args: dict) -> dict:
    dept_id = args.get("department_id", "")
    if not dept_id:
        return {"success": False, "error": "department_id is required"}
    result = _lark_api("GET", f"/contact/v3/departments/{dept_id}")
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "department": result.get("data", {}).get("department", {})}


def list_departments(args: dict) -> dict:
    parent_id = args.get("parent_department_id", "0")
    page_size = args.get("page_size", 50)
    page_token = args.get("page_token", "")
    params = f"?parent_department_id={parent_id}&page_size={page_size}"
    if page_token:
        params += f"&page_token={page_token}"
    result = _lark_api("GET", f"/contact/v3/departments{params}")
    if "error" in result:
        return {"success": False, **result}
    data = result.get("data", {})
    return {"success": True, "departments": data.get("items", []),
            "has_more": data.get("has_more", False)}


def run(args: dict) -> dict:
    action = args.pop("action", "get_user")
    handlers = {
        "get_user": get_user,
        "list_users": list_users,
        "get_department": get_department,
        "list_departments": list_departments,
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
