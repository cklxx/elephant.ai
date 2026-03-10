#!/usr/bin/env python3
"""json-render-templates skill — JSON render 协议模板生成。

生成 flowchart, form, dashboard, cards, gallery, table, kanban 等 UI 模板。
"""

from __future__ import annotations

from pathlib import Path
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv
from skill_runner.cli_contract import parse_cli_args, render_result

load_repo_dotenv(__file__)

import json

_TEMPLATES = {
    "flowchart": {
        "type": "flowchart",
        "nodes": [
            {"id": "start", "label": "开始", "type": "start"},
            {"id": "step1", "label": "步骤 1"},
            {"id": "end", "label": "结束", "type": "end"},
        ],
        "edges": [
            {"from": "start", "to": "step1"},
            {"from": "step1", "to": "end"},
        ],
    },
    "form": {
        "type": "form",
        "title": "表单标题",
        "fields": [
            {"name": "name", "label": "姓名", "type": "text", "required": True},
            {"name": "email", "label": "邮箱", "type": "email", "required": True},
            {"name": "message", "label": "留言", "type": "textarea"},
        ],
        "submit": {"label": "提交", "action": "submit"},
    },
    "dashboard": {
        "type": "dashboard",
        "title": "数据面板",
        "widgets": [
            {"type": "metric", "label": "总用户", "value": 0, "unit": "人"},
            {"type": "metric", "label": "活跃率", "value": 0, "unit": "%"},
            {"type": "chart", "chart_type": "line", "data": []},
        ],
    },
    "cards": {
        "type": "cards",
        "items": [
            {"title": "卡片 1", "description": "描述", "image": "", "actions": [{"label": "查看"}]},
        ],
    },
    "table": {
        "type": "table",
        "columns": [
            {"key": "name", "label": "名称"},
            {"key": "value", "label": "值"},
            {"key": "status", "label": "状态"},
        ],
        "rows": [],
    },
}


def generate(args: dict) -> dict:
    template_type = args.get("template", "")
    if not template_type:
        return {"success": True, "available": list(_TEMPLATES.keys()), "message": "specify template type"}
    template = _TEMPLATES.get(template_type)
    if not template:
        return {"success": False, "error": f"unknown template: {template_type}, available: {list(_TEMPLATES.keys())}"}

    # Apply customizations
    result = json.loads(json.dumps(template))  # deep copy
    if args.get("title"):
        result["title"] = args["title"]
    if args.get("data"):
        if "rows" in result:
            result["rows"] = args["data"]
        elif "items" in result:
            result["items"] = args["data"]

    return {"success": True, "template": result, "type": template_type}


def run(args: dict) -> dict:
    action = args.pop("action", "generate")
    if action == "generate":
        return generate(args)
    if action == "list":
        return {"success": True, "templates": list(_TEMPLATES.keys())}
    return {"success": False, "error": f"unknown action: {action}"}


def main() -> None:
    args = parse_cli_args(sys.argv[1:])
    result = run(args)
    stdout_text, stderr_text, exit_code = render_result(result)
    if stdout_text:
        sys.stdout.write(stdout_text)
        if not stdout_text.endswith("\n"):
            sys.stdout.write("\n")
    if stderr_text:
        sys.stderr.write(stderr_text)
        if not stderr_text.endswith("\n"):
            sys.stderr.write("\n")
    sys.exit(exit_code)


if __name__ == "__main__":
    main()
