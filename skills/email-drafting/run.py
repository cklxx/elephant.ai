#!/usr/bin/env python3
"""email-drafting skill — 邮件撰写辅助。

收集邮件要素，输出结构化输入供 LLM 撰写。
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
import sys


def collect(args: dict) -> dict:
    """收集邮件要素。"""
    purpose = args.get("purpose", "")
    if not purpose:
        return {"success": False, "error": "purpose is required"}

    return {
        "success": True,
        "elements": {
            "purpose": purpose,
            "recipient": args.get("recipient", ""),
            "tone": args.get("tone", "professional"),
            "language": args.get("language", "zh"),
            "context": args.get("context", ""),
            "key_points": args.get("key_points", []),
            "cta": args.get("cta", ""),
            "is_reply": args.get("is_reply", False),
            "thread": args.get("thread", ""),
        },
        "draft_prompt": (
            "请基于以上要素撰写邮件，注意：\n"
            "1. 主题行简洁明确（<60字符）\n"
            "2. 第一段点明目的\n"
            "3. 正文按逻辑分段，要点用列表\n"
            "4. 结尾有明确 CTA（行动号召）\n"
            "5. 语气匹配 tone 设定\n"
            "6. 如是回复，引用关键上下文"
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
