#!/usr/bin/env python3
"""web-page-editing skill — HTML 内容编辑。

解析/修改/生成 HTML 页面内容（纯 Python，无外部依赖）。
"""

from __future__ import annotations

import html
import json
import re
import sys

_TEMPLATES = {
    "landing": """<!DOCTYPE html>
<html lang="zh">
<head><meta charset="UTF-8"><title>{title}</title>
<style>body{{font-family:system-ui;max-width:800px;margin:0 auto;padding:2rem}}</style>
</head>
<body>
<h1>{title}</h1>
<p>{description}</p>
{sections}
</body>
</html>""",
    "report": """<!DOCTYPE html>
<html lang="zh">
<head><meta charset="UTF-8"><title>{title}</title>
<style>body{{font-family:system-ui;max-width:900px;margin:0 auto;padding:2rem}}
table{{border-collapse:collapse;width:100%}}td,th{{border:1px solid #ddd;padding:8px}}</style>
</head>
<body>
<h1>{title}</h1>
<p>生成时间: {date}</p>
{content}
</body>
</html>""",
}


def extract(args: dict) -> dict:
    """从 HTML 中提取文本内容。"""
    html_content = args.get("html", "")
    if not html_content:
        return {"success": False, "error": "html is required"}

    # Strip tags
    text = re.sub(r"<[^>]+>", "", html_content)
    text = html.unescape(text).strip()

    # Extract title
    title_match = re.search(r"<title>(.*?)</title>", html_content, re.IGNORECASE)
    title = title_match.group(1) if title_match else ""

    # Extract headings
    headings = re.findall(r"<h[1-6][^>]*>(.*?)</h[1-6]>", html_content, re.IGNORECASE)
    headings = [re.sub(r"<[^>]+>", "", h) for h in headings]

    # Extract links
    links = re.findall(r'<a[^>]+href="([^"]*)"[^>]*>(.*?)</a>', html_content, re.IGNORECASE)

    return {
        "success": True,
        "text": text[:10000],
        "title": title,
        "headings": headings,
        "links": [{"url": u, "text": re.sub(r"<[^>]+>", "", t)} for u, t in links],
    }


def generate(args: dict) -> dict:
    """从模板生成 HTML。"""
    template_name = args.get("template", "landing")
    template = _TEMPLATES.get(template_name)
    if not template:
        return {"success": False, "error": f"unknown template: {template_name}, available: {list(_TEMPLATES.keys())}"}

    import time
    result_html = template.format(
        title=args.get("title", "Untitled"),
        description=args.get("description", ""),
        sections=args.get("sections", ""),
        content=args.get("content", ""),
        date=time.strftime("%Y-%m-%d %H:%M"),
    )
    return {"success": True, "html": result_html, "template": template_name}


def run(args: dict) -> dict:
    action = args.pop("action", "extract")
    handlers = {"extract": extract, "generate": generate}
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
