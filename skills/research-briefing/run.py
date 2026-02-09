#!/usr/bin/env python3
"""research-briefing skill — 调研简报生成。

收集调研数据，输出结构化简报模板供 LLM 填写。
"""

from __future__ import annotations

import json
import sys
from pathlib import Path

_SCRIPTS = Path(__file__).resolve().parent.parent.parent / "scripts"
sys.path.insert(0, str(_SCRIPTS))

from cli.tavily.tavily_search import tavily_search


def collect(args: dict) -> dict:
    topic = args.get("topic", "")
    if not topic:
        return {"success": False, "error": "topic is required"}

    questions = args.get("questions", [f"What is {topic}?", f"{topic} pros and cons"])
    audience = args.get("audience", "technical")

    # Search for each question
    search_results = []
    for q in questions[:5]:
        r = tavily_search(q, max_results=3)
        search_results.append({"question": q, **r})

    return {
        "success": True,
        "topic": topic,
        "audience": audience,
        "questions": questions,
        "search_results": search_results,
        "briefing_prompt": (
            "请基于以上搜索结果，生成调研简报：\n\n"
            f"## 调研主题：{topic}\n"
            f"## 受众：{audience}\n\n"
            "### 结构：\n"
            "1. **摘要**（3-5 句，核心发现）\n"
            "2. **关键问题与发现**（每个问题的事实/假设/证据）\n"
            "3. **不确定性与空白**（哪些问题尚无定论）\n"
            "4. **建议与下一步**（实验/访谈/数据拉取建议）\n"
            "5. **参考来源**"
        ),
    }


def run(args: dict) -> dict:
    action = args.pop("action", "collect")
    if action == "collect":
        return collect(args)
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
