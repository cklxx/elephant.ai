#!/usr/bin/env python3
"""a2ui-templates skill — A2UI 组件模板生成。

生成 flowchart, form, dashboard, cards, gallery 等 A2UI 组件树。
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

_TEMPLATES = {
    "flowchart": {
        "component": "Flowchart",
        "props": {
            "nodes": [{"id": "1", "data": {"label": "节点 1"}, "position": {"x": 0, "y": 0}}],
            "edges": [],
        },
    },
    "form": {
        "component": "Form",
        "props": {
            "fields": [{"name": "input1", "label": "输入", "type": "text"}],
            "onSubmit": "handleSubmit",
        },
    },
    "dashboard": {
        "component": "Dashboard",
        "props": {
            "widgets": [{"type": "stat", "label": "指标", "value": "0"}],
        },
    },
    "cards": {
        "component": "CardList",
        "props": {
            "items": [{"title": "卡片", "content": "内容"}],
        },
    },
    "gallery": {
        "component": "Gallery",
        "props": {
            "images": [],
            "columns": 3,
        },
    },
}


def generate(args: dict) -> dict:
    template_type = args.get("template", "")
    if not template_type:
        return {"success": True, "available": list(_TEMPLATES.keys())}
    template = _TEMPLATES.get(template_type)
    if not template:
        return {"success": False, "error": f"unknown: {template_type}, available: {list(_TEMPLATES.keys())}"}
    result = json.loads(json.dumps(template))
    if args.get("data"):
        for key in ("items", "images", "widgets", "fields", "nodes"):
            if key in result.get("props", {}):
                result["props"][key] = args["data"]
                break
    return {"success": True, "component": result, "type": template_type}


def run(args: dict) -> dict:
    action = args.pop("action", "generate")
    if action == "generate":
        return generate(args)
    if action == "list":
        return {"success": True, "templates": list(_TEMPLATES.keys())}
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
