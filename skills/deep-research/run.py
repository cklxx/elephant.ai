#!/usr/bin/env python3
"""deep-research skill — 多源检索 + 证据汇编。

LLM 调用方式:
    bash: python3 skills/deep-research/run.py '{"topic":"...", "queries":["q1","q2"], "max_results":5}'

输入:
    topic       (str)  研究主题
    queries     (list) 搜索关键词列表（可选，默认从 topic 生成 3 条）
    max_results (int)  每条 query 返回的结果数（默认 5）
    depth       (str)  "basic" | "advanced"（默认 basic）
    fetch_urls  (list) 额外要抓取全文的 URL（可选）

输出 JSON:
    {
      "topic": "...",
      "searches": [
        {"query": "q1", "source": "tavily", "results": [...]},
        ...
      ],
      "fetched_pages": [
        {"url": "...", "title": "...", "content": "..."}
      ],
      "summary_prompt": "基于以上 N 条来源，请..."
    }
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
from pathlib import Path

# Add scripts/ to path for CLI imports
_SCRIPTS = Path(__file__).resolve().parent.parent.parent / "scripts"
sys.path.insert(0, str(_SCRIPTS))

from cli.tavily.tavily_search import tavily_search


def _fetch_page(url: str, max_bytes: int = 200_000) -> dict:
    """Fetch a URL and return plain text content (best-effort)."""
    req = urllib.request.Request(url, headers={"User-Agent": "Mozilla/5.0"})
    try:
        with urllib.request.urlopen(req, timeout=15) as resp:
            raw = resp.read(max_bytes).decode("utf-8", errors="replace")
    except (urllib.error.URLError, OSError):
        return {"url": url, "title": "", "content": "", "error": "fetch failed"}

    # Minimal HTML → text: strip tags
    import re
    text = re.sub(r"<script[^>]*>.*?</script>", "", raw, flags=re.S)
    text = re.sub(r"<style[^>]*>.*?</style>", "", text, flags=re.S)
    text = re.sub(r"<[^>]+>", " ", text)
    text = re.sub(r"\s+", " ", text).strip()

    # Extract title
    title_match = re.search(r"<title[^>]*>(.*?)</title>", raw, re.S | re.I)
    title = title_match.group(1).strip() if title_match else ""

    return {"url": url, "title": title, "content": text[:10000]}


def _generate_queries(topic: str) -> list[str]:
    """Generate diverse search queries from a topic."""
    return [
        topic,
        f"{topic} best practices 2026",
        f"{topic} comparison analysis",
    ]


def run(args: dict) -> dict:
    topic = args.get("topic", "")
    if not topic:
        return {"success": False, "error": "topic is required"}

    queries = args.get("queries") or _generate_queries(topic)
    max_results = args.get("max_results", 5)
    depth = args.get("depth", "basic")
    fetch_urls = args.get("fetch_urls", [])

    # Execute searches
    searches = []
    for q in queries:
        result = tavily_search(q, max_results=max_results, search_depth=depth)
        searches.append({"query": q, **result})

    # Fetch additional URLs if requested
    fetched = [_fetch_page(u) for u in fetch_urls] if fetch_urls else []

    # Collect all unique sources
    all_sources = set()
    for s in searches:
        for r in s.get("results", []):
            all_sources.add(r.get("url", ""))
    all_sources.discard("")

    return {
        "success": True,
        "topic": topic,
        "searches": searches,
        "fetched_pages": fetched,
        "total_sources": len(all_sources),
        "summary_prompt": (
            f"基于以上 {len(all_sources)} 条来源，请按"
            f"「问题→发现/证据→置信度→影响/建议」结构整理研究报告。"
            f"区分事实/假设/风险，对关键决策给出选项对比。"
        ),
    }


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
