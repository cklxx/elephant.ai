#!/usr/bin/env python3
"""bitable-data skill — 飞书多维表格管理。"""

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
import time

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


def _resolve_app_token(args: dict) -> str:
    return str(args.get("app_token", "")).strip() or str(os.environ.get("LARK_BITABLE_APP_TOKEN", "")).strip()


def _extract_app_token(data: dict) -> str:
    return str(
        data.get("app_token")
        or data.get("token")
        or data.get("app", {}).get("app_token")
        or ""
    ).strip()


def _is_transient_failure(failure: dict) -> bool:
    text = str(failure.get("error", "")).lower()
    status = int(failure.get("http_status", 0) or 0)
    return (
        "timed out" in text
        or "temporarily unavailable" in text
        or "connection reset" in text
        or status in {429, 500, 502, 503, 504}
    )


def _create_temp_app(args: dict) -> tuple[str, dict | None]:
    name_prefix = str(args.get("app_name", "")).strip() or "elephant-skill"
    last_failure: dict | None = None
    for attempt in range(2):
        name = f"{name_prefix}-{int(time.time())}-{attempt}"
        result = _lark_api("POST", "/bitable/v1/apps", {"name": name})
        failure = _api_failure(result)
        if failure:
            last_failure = failure
            if _is_transient_failure(failure) and attempt == 0:
                continue
            return "", failure

        app_token = _extract_app_token(result.get("data", {}))
        if app_token:
            return app_token, None
        last_failure = {"success": False, "error": "failed to create bitable app: app_token missing"}
    return "", (last_failure or {"success": False, "error": "failed to create bitable app"})


def _is_invalid_app_failure(failure: dict) -> bool:
    code = failure.get("code")
    if isinstance(code, int) and code in {91402, 91403, 1254040}:
        return True
    text = str(failure.get("error", "")).lower()
    return "notexist" in text or "app_token" in text and ("invalid" in text or "not found" in text)


def list_tables(args: dict) -> dict:
    app_token = _resolve_app_token(args)
    auto_create = bool(args.get("auto_create_app", True))
    app_token_source = "provided"
    if not app_token:
        if not auto_create:
            return {"success": False, "error": "app_token is required"}
        app_token, create_err = _create_temp_app(args)
        if create_err:
            return create_err
        app_token_source = "auto_created"

    result = _lark_api("GET", f"/bitable/v1/apps/{app_token}/tables")
    failure = _api_failure(result)
    if failure and _is_transient_failure(failure):
        result = _lark_api("GET", f"/bitable/v1/apps/{app_token}/tables")
        failure = _api_failure(result)
    if failure and auto_create and app_token_source != "auto_created" and _is_invalid_app_failure(failure):
        app_token_retry, create_err = _create_temp_app(args)
        if create_err:
            return failure
        retry = _lark_api("GET", f"/bitable/v1/apps/{app_token_retry}/tables")
        retry_failure = _api_failure(retry)
        if retry_failure and _is_transient_failure(retry_failure):
            retry = _lark_api("GET", f"/bitable/v1/apps/{app_token_retry}/tables")
            retry_failure = _api_failure(retry)
        if retry_failure:
            return retry_failure
        tables = retry.get("data", {}).get("items", [])
        return {
            "success": True,
            "tables": tables,
            "count": len(tables),
            "app_token": app_token_retry,
            "app_token_source": "auto_created",
            "recovered_from_app_token": app_token,
        }
    if failure:
        return failure

    tables = result.get("data", {}).get("items", [])
    return {
        "success": True,
        "tables": tables,
        "count": len(tables),
        "app_token": app_token,
        "app_token_source": app_token_source,
    }


def list_records(args: dict) -> dict:
    app_token = _resolve_app_token(args)
    table_id = args.get("table_id", "")
    if not app_token or not table_id:
        return {"success": False, "error": "app_token and table_id are required"}

    query: dict[str, str | int] = {}
    if args.get("page_size"):
        query["page_size"] = args["page_size"]
    if args.get("page_token"):
        query["page_token"] = args["page_token"]

    result = _lark_api(
        "GET",
        f"/bitable/v1/apps/{app_token}/tables/{table_id}/records",
        query=query or None,
    )
    failure = _api_failure(result)
    if failure:
        return failure
    records = result.get("data", {}).get("items", [])
    return {"success": True, "records": records, "count": len(records)}


def create_record(args: dict) -> dict:
    app_token = _resolve_app_token(args)
    table_id = args.get("table_id", "")
    fields = args.get("fields", {})
    if not app_token or not table_id:
        return {"success": False, "error": "app_token and table_id are required"}
    if not fields:
        return {"success": False, "error": "fields is required"}

    body = {"fields": fields}
    result = _lark_api("POST", f"/bitable/v1/apps/{app_token}/tables/{table_id}/records", body)
    failure = _api_failure(result)
    if failure:
        return failure
    record = result.get("data", {}).get("record", {})
    return {"success": True, "record": record, "message": "记录已创建"}


def update_record(args: dict) -> dict:
    app_token = _resolve_app_token(args)
    table_id = args.get("table_id", "")
    record_id = args.get("record_id", "")
    fields = args.get("fields", {})
    if not all([app_token, table_id, record_id]):
        return {"success": False, "error": "app_token, table_id, and record_id are required"}
    if not fields:
        return {"success": False, "error": "fields is required"}

    body = {"fields": fields}
    result = _lark_api("PUT", f"/bitable/v1/apps/{app_token}/tables/{table_id}/records/{record_id}", body)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "message": f"记录 {record_id} 已更新"}


def delete_record(args: dict) -> dict:
    app_token = _resolve_app_token(args)
    table_id = args.get("table_id", "")
    record_id = args.get("record_id", "")
    if not all([app_token, table_id, record_id]):
        return {"success": False, "error": "app_token, table_id, and record_id are required"}

    result = _lark_api("DELETE", f"/bitable/v1/apps/{app_token}/tables/{table_id}/records/{record_id}")
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "message": f"记录 {record_id} 已删除"}


def list_fields(args: dict) -> dict:
    app_token = _resolve_app_token(args)
    table_id = args.get("table_id", "")
    if not app_token or not table_id:
        return {"success": False, "error": "app_token and table_id are required"}

    result = _lark_api("GET", f"/bitable/v1/apps/{app_token}/tables/{table_id}/fields")
    failure = _api_failure(result)
    if failure:
        return failure
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
