#!/usr/bin/env python3
"""ppt-deck skill — PPT 大纲与结构化输入收集。

收集演示需求，输出结构化大纲供 LLM 生成内容 + python-pptx 生成文件。
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

_STORY_TEMPLATES = {
    "scqa": ["Situation", "Complication", "Question", "Answer"],
    "pyramid": ["Conclusion", "Supporting 1", "Supporting 2", "Supporting 3", "Details"],
    "before_after_bridge": ["Before (现状)", "After (愿景)", "Bridge (方案)"],
}

_PAGE_TYPES = [
    "cover", "tl_dr", "background", "problem", "goals",
    "solution", "comparison", "timeline", "metrics", "cta", "appendix",
]


def outline(args: dict) -> dict:
    """生成 PPT 大纲结构。"""
    topic = args.get("topic", "")
    if not topic:
        return {"success": False, "error": "topic is required"}

    template = args.get("template", "scqa")
    sections = _STORY_TEMPLATES.get(template, _STORY_TEMPLATES["scqa"])

    pages = []
    pages.append({"type": "cover", "title": topic, "subtitle": args.get("subtitle", "")})
    pages.append({"type": "tl_dr", "title": "TL;DR", "points": []})
    for s in sections:
        pages.append({"type": "content", "title": s, "points": []})
    pages.append({"type": "cta", "title": "下一步", "points": []})

    return {
        "success": True,
        "topic": topic,
        "audience": args.get("audience", ""),
        "duration": args.get("duration", ""),
        "template": template,
        "pages": pages,
        "page_types": _PAGE_TYPES,
        "brand": {
            "font": args.get("font", ""),
            "colors": args.get("colors", []),
            "logo": args.get("logo", ""),
        },
        "constraints": {
            "rule_10_20_30": args.get("rule_10_20_30", False),
            "max_pages": args.get("max_pages", 20),
            "min_font_size": args.get("min_font_size", 18),
        },
        "outline_prompt": (
            "请基于以上大纲结构和品牌规范，为每一页填充具体内容：\n"
            "1. 每页标题为「结论句」而非主题词\n"
            "2. 每页 3-5 个要点\n"
            "3. 数据/图表用占位符标注\n"
            "4. 注意 10/20/30 规则（如适用）\n"
            "5. 最终输出 JSON 格式的完整页面列表"
        ),
    }


def list_templates(args: dict) -> dict:
    return {
        "success": True,
        "story_templates": {k: v for k, v in _STORY_TEMPLATES.items()},
        "page_types": _PAGE_TYPES,
    }


def run(args: dict) -> dict:
    action = args.pop("action", "outline")
    handlers = {"outline": outline, "list": list_templates}
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
