#!/usr/bin/env python3
"""moltbook-posting skill — Moltbook 社交网络 API 封装。

支持发帖、浏览、评论、投票、搜索。
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
import sys
import urllib.error
import urllib.parse
import urllib.request


def _normalize_base(base: str) -> str:
    base = base.strip().rstrip("/")
    if not base.endswith("/api"):
        base = f"{base}/api"
    return base


def _load_from_alex_config() -> tuple[str, str]:
    config_path = Path.home() / ".alex" / "config.yaml"
    if not config_path.is_file():
        return "", ""
    try:
        text = config_path.read_text(encoding="utf-8")
    except Exception:
        return "", ""
    key = ""
    base = ""
    for line in text.splitlines():
        stripped = line.strip()
        if stripped.startswith("moltbook_api_key:"):
            key = stripped.split("moltbook_api_key:", 1)[1].strip().strip("\"'")
        elif stripped.startswith("moltbook_base_url:"):
            base = stripped.split("moltbook_base_url:", 1)[1].strip().strip("\"'")
    return key, base


_BASE = _normalize_base(os.environ.get("MOLTBOOK_API_URL", "https://moltbook.ai/api"))
_API_KEY = os.environ.get("MOLTBOOK_API_KEY", "")

if not _API_KEY:
    cfg_key, cfg_base = _load_from_alex_config()
    if cfg_key:
        _API_KEY = cfg_key
    if not os.environ.get("MOLTBOOK_API_URL") and cfg_base:
        _BASE = _normalize_base(cfg_base)


def _api(method: str, path: str, body: dict | None = None) -> dict:
    if not _API_KEY:
        return {"error": "MOLTBOOK_API_KEY not set"}
    url = f"{_BASE}{path}"
    headers = {"Authorization": f"Bearer {_API_KEY}", "Content-Type": "application/json"}
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            return json.loads(resp.read().decode())
    except urllib.error.URLError as exc:
        return {"error": str(exc)}


def post(args: dict) -> dict:
    content = args.get("content", "")
    if not content:
        return {"success": False, "error": "content is required"}
    result = _api("POST", "/posts", {"content": content, "tags": args.get("tags", [])})
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "post": result.get("data", {}), "message": "posted"}


def feed(args: dict) -> dict:
    limit = args.get("limit", 20)
    result = _api("GET", f"/feed?limit={limit}")
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "posts": result.get("data", []), "count": len(result.get("data", []))}


def search(args: dict) -> dict:
    query = args.get("query", "")
    if not query:
        return {"success": False, "error": "query is required"}
    result = _api("GET", f"/search?q={urllib.parse.quote(query)}")
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "results": result.get("data", [])}


def run(args: dict) -> dict:
    action = args.pop("action", "feed")
    handlers = {"post": post, "feed": feed, "search": search}
    handler = handlers.get(action)
    if not handler:
        return {"success": False, "error": f"unknown action: {action}"}
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
    sys.exit(0 if result.get("success") else 1)


if __name__ == "__main__":
    main()
