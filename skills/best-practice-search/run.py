#!/usr/bin/env python3
"""best-practice-search skill — 搜索最佳实践。

多源搜索 + 本地 docs/ 检索，汇编最佳实践供 LLM 综合。
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
from pathlib import Path

_SCRIPTS = Path(__file__).resolve().parent.parent.parent / "scripts"
sys.path.insert(0, str(_SCRIPTS))

from cli.tavily.tavily_search import tavily_search


def search(args: dict) -> dict:
    topic = args.get("topic", "")
    if not topic:
        return {"success": False, "error": "topic is required"}

    # Web search
    queries = [
        f"{topic} best practices",
        f"{topic} 最佳实践 production",
    ]
    web_results = []
    for q in queries:
        r = tavily_search(q, max_results=args.get("max_results", 5))
        web_results.append({"query": q, **r})

    # Local docs search
    docs_dir = args.get("docs_dir", "docs")
    local_results = []
    if Path(docs_dir).exists():
        try:
            out = subprocess.run(
                ["grep", "-rl", topic, docs_dir],
                capture_output=True, text=True, timeout=10,
            )
            files = [f for f in out.stdout.strip().split("\n") if f]
            for f in files[:5]:
                content = Path(f).read_text(encoding="utf-8", errors="replace")[:2000]
                local_results.append({"file": f, "content": content})
        except (subprocess.TimeoutExpired, FileNotFoundError):
            pass

    return {
        "success": True,
        "topic": topic,
        "web_results": web_results,
        "local_results": local_results,
        "synthesis_prompt": (
            f"请基于以上 {len(web_results)} 组搜索结果和 {len(local_results)} 个本地文档，"
            "整理该主题的最佳实践：\n"
            "1. TL;DR（3-5 条核心实践）\n"
            "2. 共识 vs 分歧（多源一致的 vs 有争议的）\n"
            "3. 适用边界（什么场景适用/不适用）\n"
            "4. 可执行建议（具体步骤）\n"
            "5. 参考来源"
        ),
    }


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
