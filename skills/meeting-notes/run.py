#!/usr/bin/env python3
"""meeting-notes skill — 会议记录结构化。

收集原始会议笔记，输出结构化格式供 LLM 整理。
"""

from __future__ import annotations

import json
import sys
import time
from pathlib import Path


def collect(args: dict) -> dict:
    """收集会议原始笔记并提供结构化模板。"""
    raw_notes = args.get("notes", "")
    file_path = args.get("file", "")

    if file_path and Path(file_path).exists():
        raw_notes = Path(file_path).read_text(encoding="utf-8")

    if not raw_notes:
        return {"success": False, "error": "notes or file is required"}

    return {
        "success": True,
        "raw_notes": raw_notes,
        "word_count": len(raw_notes),
        "meeting_info": {
            "date": args.get("date", time.strftime("%Y-%m-%d")),
            "title": args.get("title", ""),
            "attendees": args.get("attendees", []),
            "duration": args.get("duration", ""),
        },
        "format_prompt": (
            "请将以上会议原始笔记整理为结构化纪要：\n\n"
            "## 会议概要\n一句话总结\n\n"
            "## 关键决策\n- 决策 1（决策人）\n\n"
            "## 行动项\n| 项目 | 负责人 | 截止时间 | 状态 |\n\n"
            "## 讨论要点\n按主题分组\n\n"
            "## 待澄清问题\n尚未达成共识的议题\n\n"
            "## 风险与关注\n需要跟进的风险点"
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
