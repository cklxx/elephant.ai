#!/usr/bin/env python3
"""calendar-management skill — Lark 日历事件管理。

通过 Lark Open API 管理日历事件。需要 LARK_APP_ID / LARK_APP_SECRET 环境变量。
当前为框架实现，Lark API 调用待对接。
"""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request


def _lark_api(method: str, path: str, body: dict | None = None) -> dict:
    """Call Lark Open API (placeholder — needs tenant_access_token)."""
    base = "https://open.feishu.cn/open-apis"
    token = os.environ.get("LARK_TENANT_TOKEN", "")
    if not token:
        return {"error": "LARK_TENANT_TOKEN not set, calendar operations unavailable"}

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


def create_event(args: dict) -> dict:
    title = args.get("title", "")
    start = args.get("start", "")
    if not title or not start:
        return {"success": False, "error": "title and start are required"}

    # Build Lark calendar event payload
    body = {
        "summary": title,
        "start_time": {"timestamp": start},
        "description": args.get("description", ""),
    }

    if args.get("attendees"):
        body["attendees"] = [{"type": "user", "user_id": a} for a in args["attendees"]]

    result = _lark_api("POST", "/calendar/v4/calendars/primary/events", body)
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "event": result.get("data", {}), "message": f"事件「{title}」已创建"}


def query_events(args: dict) -> dict:
    start = args.get("start", "")
    end = args.get("end", "")
    if not start:
        return {"success": False, "error": "start date is required"}

    params = f"?start_time={start}"
    if end:
        params += f"&end_time={end}"

    result = _lark_api("GET", f"/calendar/v4/calendars/primary/events{params}")
    if "error" in result:
        return {"success": False, **result}
    events = result.get("data", {}).get("items", [])
    return {"success": True, "events": events, "count": len(events)}


def delete_event(args: dict) -> dict:
    event_id = args.get("event_id", "")
    if not event_id:
        return {"success": False, "error": "event_id is required"}

    result = _lark_api("DELETE", f"/calendar/v4/calendars/primary/events/{event_id}")
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "message": f"事件 {event_id} 已删除"}


def run(args: dict) -> dict:
    action = args.pop("action", "query")

    handlers = {
        "create": create_event,
        "query": query_events,
        "delete": delete_event,
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
