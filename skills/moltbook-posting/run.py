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

import http.client
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request


def _normalize_base(base: str) -> str:
    base = base.strip().rstrip("/")
    if "/api" in base:
        return base
    return f"{base}/api"


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



def _canonicalize_base(base: str) -> str:
    base = base.strip().rstrip("/")
    if base.endswith("/api"):
        return f"{base}/v1"
    if base.endswith("/api/v1"):
        return base
    if "/api/" in base:
        return base
    return f"{base}/api/v1"


def _resolve_base() -> str:
    # Prefer config/env if they explicitly include /api
    base = _BASE
    if "/api" in base:
        return _canonicalize_base(base)
    # Otherwise pick the most stable endpoint for auth'd requests
    return "https://www.moltbook.com/api/v1"


_BASE = _resolve_base()


def _api(method: str, path: str, body: dict | None = None) -> dict:
    if not _API_KEY:
        return {"error": "MOLTBOOK_API_KEY not set"}
    url = f"{_BASE}{path}"
    headers = {
        "Authorization": f"Bearer {_API_KEY}",
        "Content-Type": "application/json",
        "User-Agent": "moltbook-skill/1.0",
        "Accept": "application/json",
        "Connection": "close",
    }
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        return _read_response(req, timeout=15)
    except urllib.error.HTTPError as exc:
        payload_text = ""
        try:
            payload_text = exc.read().decode()
            payload = json.loads(payload_text)
        except Exception:
            payload = {"message": payload_text or str(exc)}
        if "error" not in payload:
            payload["error"] = payload.get("message") or f"HTTP {exc.code}"
        payload["http_status"] = exc.code
        return payload
    except urllib.error.URLError as exc:
        return {"error": str(exc)}


def _read_response(req: urllib.request.Request, timeout: int) -> dict:
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            return json.loads(resp.read().decode())
    except http.client.RemoteDisconnected:
        return _retry_with_https(req, timeout=timeout)


def _retry_with_https(req: urllib.request.Request, timeout: int) -> dict:
    import ssl

    ctx = ssl.create_default_context()
    try:
        with urllib.request.urlopen(req, timeout=timeout, context=ctx) as resp:
            return json.loads(resp.read().decode())
    except http.client.RemoteDisconnected:
        return _retry_as_legacy(req, timeout=timeout)
    except Exception as exc:
        return {"error": str(exc)}


def _retry_as_legacy(req: urllib.request.Request, timeout: int) -> dict:
    parsed = urllib.parse.urlparse(req.full_url)
    path = parsed.path
    if parsed.query:
        path = f"{path}?{parsed.query}"
    conn = http.client.HTTPSConnection(parsed.netloc, timeout=timeout)
    body = req.data
    headers = dict(req.header_items())
    try:
        conn.request(req.get_method(), path, body=body, headers=headers)
        resp = conn.getresponse()
        payload = resp.read().decode()
        try:
            return json.loads(payload)
        except Exception:
            return {"error": payload or f"HTTP {resp.status}"}
    except Exception as exc:
        return {"error": str(exc)}
    finally:
        try:
            conn.close()
        except Exception:
            pass


def post(args: dict) -> dict:
    title = str(args.get("title", "")).strip()
    content = args.get("content", "")
    if not title:
        return {"success": False, "error": "title is required"}
    if not content:
        return {"success": False, "error": "content is required"}
    submolt = str(args.get("submolt", "general")).strip() or "general"
    payload = {
        "title": title,
        "content": content,
        "submolt": submolt,
        "tags": args.get("tags", []),
    }
    result = _api("POST", "/posts", payload)
    if "error" in result:
        return {"success": False, **result}
    return {"success": True, "post": result.get("data", {}), "message": "posted"}


def feed(args: dict) -> dict:
    limit = args.get("limit", 20)
    sort = args.get("sort", "hot")
    result = _api("GET", f"/posts?limit={limit}&sort={urllib.parse.quote(str(sort))}")
    if "error" in result:
        return {"success": False, **result}
    posts = result.get("data", result.get("posts", []))
    return {"success": True, "posts": posts, "count": len(posts)}


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
