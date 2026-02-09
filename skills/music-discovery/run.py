#!/usr/bin/env python3
"""music-discovery skill — 音乐发现。

通过 iTunes Search API 搜索歌曲/专辑/艺人。
"""

from __future__ import annotations

import json
import sys
import urllib.error
import urllib.parse
import urllib.request


_ITUNES_API = "https://itunes.apple.com/search"


def search(args: dict) -> dict:
    query = args.get("query", "")
    if not query:
        return {"success": False, "error": "query is required"}

    params = {
        "term": query,
        "media": args.get("media", "music"),
        "limit": min(args.get("limit", 10), 25),
        "country": args.get("country", "CN"),
    }
    url = f"{_ITUNES_API}?{urllib.parse.urlencode(params)}"

    try:
        req = urllib.request.Request(url, headers={"User-Agent": "alex-music/1.0"})
        with urllib.request.urlopen(req, timeout=10) as resp:
            data = json.loads(resp.read().decode())
    except urllib.error.URLError as exc:
        return {"success": False, "error": str(exc)}

    results = []
    for item in data.get("results", []):
        results.append({
            "track": item.get("trackName", ""),
            "artist": item.get("artistName", ""),
            "album": item.get("collectionName", ""),
            "preview_url": item.get("previewUrl", ""),
            "artwork": item.get("artworkUrl100", ""),
            "genre": item.get("primaryGenreName", ""),
            "duration_ms": item.get("trackTimeMillis", 0),
        })

    return {"success": True, "query": query, "results": results, "count": len(results)}


def run(args: dict) -> dict:
    action = args.pop("action", "search")
    if action == "search":
        return search(args)
    return {"success": False, "error": f"unknown action: {action}"}


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
