#!/usr/bin/env python3
"""meeting-automation skill — 飞书视频会议管理。

通过 channel tool 的 vc actions 查询飞书视频会议。
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


def list_meetings(args: dict) -> dict:
    start_time = args.get("start_time", "")
    end_time = args.get("end_time", "")
    if not start_time or not end_time:
        return {"success": False, "error": "start_time and end_time are required"}
    page_size = args.get("page_size", 20)
    page_token = args.get("page_token", "")
    params = f"?start_time={start_time}&end_time={end_time}&page_size={page_size}"
    if page_token:
        params += f"&page_token={page_token}"
    result = _lark_api("GET", f"/vc/v1/meeting_list{params}")
    if "error" in result:
        return {"success": False, **result}
    data = result.get("data", {})
    return {"success": True, "meetings": data.get("meeting_list", []),
            "has_more": data.get("has_more", False)}


def get_meeting(args: dict) -> dict:
    meeting_id = args.get("meeting_id", "")
    if not meeting_id:
        return {"success": False, "error": "meeting_id is required"}
    result = _lark_api("GET", f"/vc/v1/meetings/{meeting_id}")
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "meeting": result.get("data", {}).get("meeting", {})}


def list_rooms(args: dict) -> dict:
    page_size = args.get("page_size", 20)
    page_token = args.get("page_token", "")
    room_level_id = args.get("room_level_id", "")
    params = f"?page_size={page_size}"
    if room_level_id:
        params += f"&room_level_id={room_level_id}"
    if page_token:
        params += f"&page_token={page_token}"
    result = _lark_api("GET", f"/vc/v1/rooms{params}")
    if "error" in result:
        return {"success": False, **result}
    data = result.get("data", {})
    return {"success": True, "rooms": data.get("rooms", []),
            "has_more": data.get("has_more", False)}


def run(args: dict) -> dict:
    action = args.pop("action", "list_meetings")
    handlers = {
        "list_meetings": list_meetings,
        "get_meeting": get_meeting,
        "list_rooms": list_rooms,
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
