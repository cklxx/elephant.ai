#!/usr/bin/env python3
"""okr-native skill — 飞书原生 OKR 管理。

通过 channel tool 的 OKR actions 查询飞书原生 OKR。
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
    base = "https://open.feishu.cn/open-apis"
    token = os.environ.get("LARK_TENANT_TOKEN", "")
    if not token:
        return {"error": "LARK_TENANT_TOKEN not set"}

    url = f"{base}{path}"
    headers = {"Authorization": f"Bearer {token}", "Content-Type": "application/json"}
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(url, data=data, headers=headers, method=method)

    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.URLError as exc:
        return {"error": str(exc)}


def list_periods(args: dict) -> dict:
    page_size = args.get("page_size", 20)
    page_token = args.get("page_token", "")
    params = f"?page_size={page_size}"
    if page_token:
        params += f"&page_token={page_token}"
    result = _lark_api("GET", f"/okr/v1/periods{params}")
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "periods": result.get("data", {}).get("items", []),
            "has_more": result.get("data", {}).get("has_more", False)}


def list_user_okrs(args: dict) -> dict:
    user_id = args.get("user_id", "")
    if not user_id:
        return {"success": False, "error": "user_id is required"}
    result = _lark_api("GET", f"/okr/v1/users/{user_id}/okrs")
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "okrs": result.get("data", {}).get("okr_list", [])}


def batch_get_okrs(args: dict) -> dict:
    okr_ids = args.get("okr_ids", [])
    if not okr_ids:
        return {"success": False, "error": "okr_ids is required"}
    ids_param = "&".join(f"okr_ids={i}" for i in okr_ids)
    result = _lark_api("GET", f"/okr/v1/okrs/batch_get?{ids_param}")
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "okrs": result.get("data", {}).get("okr_list", [])}


def run(args: dict) -> dict:
    action = args.pop("action", "list_periods")
    handlers = {
        "list_periods": list_periods,
        "list_user_okrs": list_user_okrs,
        "batch_get": batch_get_okrs,
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
