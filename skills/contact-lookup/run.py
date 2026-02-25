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

from skill_runner.lark_auth import lark_api_json


def _lark_api(method: str, path: str, *, query: dict | str | None = None) -> dict:
    return lark_api_json(method, path, query=query)


def _api_failure(result: dict) -> dict | None:
    if "error" in result:
        return {"success": False, **result}
    code = result.get("code", 0)
    if isinstance(code, int) and code != 0:
        return {"success": False, "code": code, "error": result.get("msg") or f"Lark API error {code}"}
    return None


def _is_permission_failure(failure: dict) -> bool:
    code = failure.get("code")
    if isinstance(code, int) and code in {40003, 40004, 40013, 41050, 99991400, 99991401}:
        return True
    text = str(failure.get("error", "")).lower()
    return any(
        token in text
        for token in (
            "permission",
            "authority",
            "forbidden",
            "no dept authority",
            "insufficient scope",
            "access denied",
        )
    )


def list_scopes(_: dict) -> dict:
    result = _lark_api("GET", "/contact/v3/scopes")
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "scopes": result.get("data", {})}


def _scope_fallback(action: str, failure: dict) -> dict:
    scopes = list_scopes({})
    if not scopes.get("success"):
        return failure
    return {
        "success": True,
        "source": "scope_fallback",
        "fallback_for": action,
        "warning": failure.get("error", "permission limited"),
        "scopes": scopes.get("scopes", {}),
    }


def get_user(args: dict) -> dict:
    user_id = args.get("user_id", "")
    if not user_id:
        return {"success": False, "error": "user_id is required"}
    user_id_type = args.get("user_id_type", "open_id")
    result = _lark_api("GET", f"/contact/v3/users/{user_id}", query={"user_id_type": user_id_type})
    failure = _api_failure(result)
    if failure:
        if _is_permission_failure(failure):
            return _scope_fallback("get_user", failure)
        return failure
    return {"success": True, "user": result.get("data", {}).get("user", {})}


def list_users(args: dict) -> dict:
    dept_id = args.get("department_id", "")
    if not dept_id:
        return {"success": False, "error": "department_id is required"}
    page_size = args.get("page_size", 50)
    page_token = args.get("page_token", "")
    query: dict[str, str | int] = {"department_id": dept_id, "page_size": page_size}
    if page_token:
        query["page_token"] = page_token
    result = _lark_api("GET", "/contact/v3/users", query=query)
    failure = _api_failure(result)
    if failure:
        if _is_permission_failure(failure):
            return _scope_fallback("list_users", failure)
        return failure
    data = result.get("data", {})
    return {"success": True, "users": data.get("items", []),
            "has_more": data.get("has_more", False)}


def get_department(args: dict) -> dict:
    dept_id = args.get("department_id", "")
    if not dept_id:
        return {"success": False, "error": "department_id is required"}
    result = _lark_api("GET", f"/contact/v3/departments/{dept_id}")
    failure = _api_failure(result)
    if failure:
        if _is_permission_failure(failure):
            return _scope_fallback("get_department", failure)
        return failure
    return {"success": True, "department": result.get("data", {}).get("department", {})}


def list_departments(args: dict) -> dict:
    parent_id = args.get("parent_department_id", "0")
    page_size = args.get("page_size", 50)
    page_token = args.get("page_token", "")
    query: dict[str, str | int] = {"parent_department_id": parent_id, "page_size": page_size}
    if page_token:
        query["page_token"] = page_token
    result = _lark_api("GET", "/contact/v3/departments", query=query)
    failure = _api_failure(result)
    if failure:
        if _is_permission_failure(failure):
            return _scope_fallback("list_departments", failure)
        return failure
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
        "list_scopes": list_scopes,
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
