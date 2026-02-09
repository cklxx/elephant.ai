#!/usr/bin/env python3
"""Thin CLI wrapper for Tavily Search API.

Usage:
    tavily_search.py "query"
    tavily_search.py "query" --max-results 10 --depth advanced
    tavily_search.py --json '{"query":"...", "max_results":5}'

Output: JSON to stdout.
Falls back to DuckDuckGo HTML scraping when TAVILY_API_KEY is unset.
"""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
from html.parser import HTMLParser


def tavily_search(
    query: str,
    *,
    max_results: int = 5,
    search_depth: str = "basic",
    api_key: str | None = None,
) -> dict:
    """Call Tavily API and return structured results."""
    key = api_key or os.environ.get("TAVILY_API_KEY", "")
    if not key:
        return _duckduckgo_fallback(query, max_results)

    payload = json.dumps({
        "api_key": key,
        "query": query,
        "max_results": max_results,
        "search_depth": search_depth,
        "include_answer": True,
    }).encode()

    req = urllib.request.Request(
        "https://api.tavily.com/search",
        data=payload,
        headers={"Content-Type": "application/json"},
        method="POST",
    )

    try:
        with urllib.request.urlopen(req, timeout=30) as resp:
            data = json.loads(resp.read().decode())
    except (urllib.error.URLError, json.JSONDecodeError) as exc:
        return _duckduckgo_fallback(query, max_results)

    results = []
    for r in data.get("results", []):
        results.append({
            "title": r.get("title", ""),
            "url": r.get("url", ""),
            "content": r.get("content", ""),
            "score": r.get("score", 0),
        })

    return {
        "source": "tavily",
        "query": query,
        "answer": data.get("answer", ""),
        "results": results,
        "results_count": len(results),
    }


# ── DuckDuckGo fallback ─────────────────────────────────────────


class _DDGParser(HTMLParser):
    """Minimal parser for DuckDuckGo HTML search results."""

    def __init__(self) -> None:
        super().__init__()
        self.results: list[dict] = []
        self._in_result = False
        self._in_link = False
        self._in_snippet = False
        self._current: dict = {}
        self._text_buf: list[str] = []

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        cls = dict(attrs).get("class", "")
        if tag == "div" and "result__body" in cls:
            self._in_result = True
            self._current = {}
        elif self._in_result and tag == "a" and "result__a" in cls:
            self._in_link = True
            self._current["url"] = dict(attrs).get("href", "")
            self._text_buf = []
        elif self._in_result and tag == "a" and "result__snippet" in cls:
            self._in_snippet = True
            self._text_buf = []

    def handle_endtag(self, tag: str) -> None:
        if self._in_link and tag == "a":
            self._current["title"] = "".join(self._text_buf).strip()
            self._in_link = False
        elif self._in_snippet and tag == "a":
            self._current["content"] = "".join(self._text_buf).strip()
            self._in_snippet = False
        elif self._in_result and tag == "div":
            if self._current.get("title"):
                self.results.append(self._current)
            self._in_result = False

    def handle_data(self, data: str) -> None:
        if self._in_link or self._in_snippet:
            self._text_buf.append(data)


def _duckduckgo_fallback(query: str, max_results: int) -> dict:
    """Scrape DuckDuckGo HTML as fallback when no API key."""
    url = f"https://html.duckduckgo.com/html/?q={urllib.parse.quote(query)}"
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            html = resp.read().decode("utf-8", errors="replace")
    except urllib.error.URLError:
        return {
            "source": "error",
            "query": query,
            "answer": "",
            "results": [],
            "results_count": 0,
        }

    parser = _DDGParser()
    parser.feed(html)
    results = parser.results[:max_results]
    for r in results:
        r.setdefault("score", 0)

    return {
        "source": "duckduckgo",
        "query": query,
        "answer": "",
        "results": results,
        "results_count": len(results),
    }


# ── CLI entry point ──────────────────────────────────────────────


def main() -> None:
    parser = argparse.ArgumentParser(description="Tavily web search CLI")
    parser.add_argument("query", nargs="?", help="search query")
    parser.add_argument("--max-results", type=int, default=5)
    parser.add_argument("--depth", default="basic", choices=["basic", "advanced"])
    parser.add_argument("--json", dest="json_input", help="JSON input string")
    args = parser.parse_args()

    if args.json_input:
        params = json.loads(args.json_input)
        query = params["query"]
        max_results = params.get("max_results", 5)
        depth = params.get("search_depth", "basic")
    elif args.query:
        query = args.query
        max_results = args.max_results
        depth = args.depth
    else:
        parser.error("query is required")
        return

    result = tavily_search(query, max_results=max_results, search_depth=depth)
    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
