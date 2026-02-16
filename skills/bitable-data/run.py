#!/usr/bin/env python3
"""bitable-data skill — 飞书多维表格管理。

通过 channel tool 的 bitable actions 管理飞书多维表格。
当前为框架实现，实际调用通过 channel tool 的 list_bitable_*/create_bitable_*/update_bitable_*/delete_bitable_* actions。
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
        return {"error": "LARK_TENANT_TOKEN not set, bitable operations unavailable"}

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


def list_tables(args: dict) -> dict:
    app_token = args.get("app_token", "")
    if not app_token:
        return {"success": False, "error": "app_token is required"}

    result = _lark_api("GET", f"/bitable/v1/apps/{app_token}/tables")
    if "error" in result:
        return {"success": False, **result}
    tables = result.get("data", {}).get("items", [])
    return {"success": True, "tables": tables, "count": len(tables)}


def list_records(args: dict) -> dict:
    app_token = args.get("app_token", "")
    table_id = args.get("table_id", "")
    if not app_token or not table_id:
        return {"success": False, "error": "app_token and table_id are required"}

    params = ""
    if args.get("page_size"):
        params += f"?page_size={args['page_size']}"
    if args.get("page_token"):
        sep = "&" if params else "?"
        params += f"{sep}page_token={args['page_token']}"

    result = _lark_api("GET", f"/bitable/v1/apps/{app_token}/tables/{table_id}/records{params}")
    if "error" in result:
        return {"success": False, **result}
    records = result.get("data", {}).get("items", [])
    return {"success": True, "records": records, "count": len(records)}


def create_record(args: dict) -> dict:
    app_token = args.get("app_token", "")
    table_id = args.get("table_id", "")
    fields = args.get("fields", {})
    if not app_token or not table_id:
        return {"success": False, "error": "app_token and table_id are required"}
    if not fields:
        return {"success": False, "error": "fields is required"}

    body = {"fields": fields}
    result = _lark_api("POST", f"/bitable/v1/apps/{app_token}/tables/{table_id}/records", body)
    if "error" in result:
        return {"success": False, **result}
    record = result.get("data", {}).get("record", {})
    return {"success": True, "record": record, "message": "记录已创建"}


def update_record(args: dict) -> dict:
    app_token = args.get("app_token", "")
    table_id = args.get("table_id", "")
    record_id = args.get("record_id", "")
    fields = args.get("fields", {})
    if not all([app_token, table_id, record_id]):
        return {"success": False, "error": "app_token, table_id, and record_id are required"}
    if not fields:
        return {"success": False, "error": "fields is required"}

    body = {"fields": fields}
    result = _lark_api("PUT", f"/bitable/v1/apps/{app_token}/tables/{table_id}/records/{record_id}", body)
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "message": f"记录 {record_id} 已更新"}


def delete_record(args: dict) -> dict:
    app_token = args.get("app_token", "")
    table_id = args.get("table_id", "")
    record_id = args.get("record_id", "")
    if not all([app_token, table_id, record_id]):
        return {"success": False, "error": "app_token, table_id, and record_id are required"}

    result = _lark_api("DELETE", f"/bitable/v1/apps/{app_token}/tables/{table_id}/records/{record_id}")
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "message": f"记录 {record_id} 已删除"}


def list_fields(args: dict) -> dict:
    app_token = args.get("app_token", "")
    table_id = args.get("table_id", "")
    if not app_token or not table_id:
        return {"success": False, "error": "app_token and table_id are required"}

    result = _lark_api("GET", f"/bitable/v1/apps/{app_token}/tables/{table_id}/fields")
    if "error" in result:
        return {"success": False, **result}
    fields = result.get("data", {}).get("items", [])
    return {"success": True, "fields": fields, "count": len(fields)}


def run(args: dict) -> dict:
    action = args.pop("action", "list_tables")

    handlers = {
        "list_tables": list_tables,
        "list_records": list_records,
        "create_record": create_record,
        "update_record": update_record,
        "delete_record": delete_record,
        "list_fields": list_fields,
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
