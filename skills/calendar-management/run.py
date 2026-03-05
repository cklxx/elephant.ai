#!/usr/bin/env python3
"""calendar-management skill — Lark 日历事件管理。"""

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
from datetime import datetime, timedelta, timezone

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


def _parse_ts(value: str) -> str | None:
    value = str(value).strip()
    if not value:
        return None
    if value.isdigit():
        return value

    normalized = value.replace(" ", "T")
    for fmt in ("%Y-%m-%dT%H:%M:%S", "%Y-%m-%dT%H:%M", "%Y-%m-%d"):
        try:
            dt = datetime.strptime(normalized, fmt)
            if fmt == "%Y-%m-%d":
                dt = datetime.combine(dt.date(), datetime.min.time())
            return str(int(dt.replace(tzinfo=timezone.utc).timestamp()))
        except ValueError:
            continue
    try:
        dt = datetime.fromisoformat(normalized)
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        return str(int(dt.timestamp()))
    except ValueError:
        return None


def _parse_duration_seconds(value: str | int | float | None) -> int:
    if isinstance(value, (int, float)):
        return max(int(value), 60)
    text = str(value or "60m").strip().lower()
    if text.endswith("m") and text[:-1].isdigit():
        return int(text[:-1]) * 60
    if text.endswith("h") and text[:-1].isdigit():
        return int(text[:-1]) * 3600
    if text.isdigit():
        return int(text)
    return 3600


def _resolve_calendar_id(args: dict) -> tuple[str, dict | None]:
    provided = (
        str(args.get("calendar_id", "")).strip()
        or str(args.get("calendar_token", "")).strip()
        or str(os.environ.get("LARK_CALENDAR_ID", "")).strip()
    )
    if provided:
        return provided, None

    result = _lark_api("GET", "/calendar/v4/calendars")
    failure = _api_failure(result)
    if failure:
        # Keep backward compatibility for unit tests and environments where listing calendars is blocked.
        return "primary", None

    items = result.get("data", {}).get("items", [])
    if items:
        calendar_id = str(items[0].get("calendar_id", "")).strip()
        if calendar_id:
            return calendar_id, None
    return "primary", None


def create_event(args: dict) -> dict:
    title = args.get("title", "")
    start = args.get("start", "")
    if not title or not start:
        return {"success": False, "error": "title and start are required"}

    start_ts = _parse_ts(start)
    if not start_ts:
        return {"success": False, "error": "invalid start format; use timestamp or YYYY-MM-DD[ HH:MM]"}
    duration_seconds = _parse_duration_seconds(args.get("duration"))
    end_ts = str(int(start_ts) + duration_seconds)
    calendar_id, cal_err = _resolve_calendar_id(args)
    if cal_err:
        return cal_err

    body = {
        "summary": title,
        "start_time": {"timestamp": start_ts},
        "end_time": {"timestamp": end_ts},
        "description": args.get("description", ""),
    }

    result = _lark_api("POST", f"/calendar/v4/calendars/{calendar_id}/events", body)
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "event": result.get("data", {}), "message": f"事件「{title}」已创建"}


def query_events(args: dict) -> dict:
    start = args.get("start", "")
    end = args.get("end", "")
    if not start:
        return {"success": False, "error": "start date is required"}

    start_ts = _parse_ts(start)
    if not start_ts:
        return {"success": False, "error": "invalid start format; use timestamp or YYYY-MM-DD[ HH:MM]"}
    if end:
        end_ts = _parse_ts(end)
    else:
        dt = datetime.fromtimestamp(int(start_ts), tz=timezone.utc) + timedelta(days=1)
        end_ts = str(int(dt.timestamp()))
    if not end_ts:
        return {"success": False, "error": "invalid end format; use timestamp or YYYY-MM-DD[ HH:MM]"}

    calendar_id, cal_err = _resolve_calendar_id(args)
    if cal_err:
        return cal_err

    result = _lark_api(
        "GET",
        f"/calendar/v4/calendars/{calendar_id}/events",
        query={"start_time": start_ts, "end_time": end_ts},
    )
    failure = _api_failure(result)
    if failure:
        return failure
    events = result.get("data", {}).get("items", [])
    return {"success": True, "events": events, "count": len(events)}


def delete_event(args: dict) -> dict:
    event_id = args.get("event_id", "")
    if not event_id:
        return {"success": False, "error": "event_id is required"}

    calendar_id, cal_err = _resolve_calendar_id(args)
    if cal_err:
        return cal_err
    result = _lark_api("DELETE", f"/calendar/v4/calendars/{calendar_id}/events/{event_id}")
    failure = _api_failure(result)
    if failure:
        return failure
    return {"success": True, "message": f"事件 {event_id} 已删除"}


def list_calendars(_: dict) -> dict:
    result = _lark_api("GET", "/calendar/v4/calendars")
    failure = _api_failure(result)
    if failure:
        return failure
    calendars = result.get("data", {}).get("items", [])
    return {"success": True, "calendars": calendars, "count": len(calendars)}


def run(args: dict) -> dict:
    action = args.pop("action", "query")

    handlers = {
        "create": create_event,
        "query": query_events,
        "delete": delete_event,
        "list_calendars": list_calendars,
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
