#!/usr/bin/env python3
"""okr-management skill — OKR 目标管理 CRUD。

读写 OKR YAML 文件，支持创建、查询、更新进度、回顾。
"""

from __future__ import annotations

import json
import os
import sys
import time
from pathlib import Path

_OKR_DIR = Path(os.environ.get("OKR_GOALS_ROOT", os.path.expanduser("~/.alex/okr")))


def _ensure_dir():
    _OKR_DIR.mkdir(parents=True, exist_ok=True)


def create(args: dict) -> dict:
    _ensure_dir()
    title = args.get("title", "")
    if not title:
        return {"success": False, "error": "title is required"}

    slug = title.lower().replace(" ", "-").replace("/", "-")[:50]
    ts = int(time.time())
    filename = f"{slug}-{ts}.md"
    filepath = _OKR_DIR / filename

    krs = args.get("key_results", [])
    kr_text = "\n".join(f"- [ ] {kr}" for kr in krs) if krs else "- [ ] KR1: (define)"

    content = f"""---
title: "{title}"
status: active
created: {time.strftime('%Y-%m-%d')}
updated: {time.strftime('%Y-%m-%d')}
---

# {title}

## Objective
{args.get('objective', title)}

## Key Results
{kr_text}

## Notes
{args.get('notes', '')}
"""
    filepath.write_text(content, encoding="utf-8")
    return {"success": True, "path": str(filepath), "message": f"OKR「{title}」已创建"}


def list_okrs(args: dict) -> dict:
    _ensure_dir()
    status_filter = args.get("status", "")
    okrs = []
    for f in sorted(_OKR_DIR.glob("*.md")):
        text = f.read_text(encoding="utf-8")
        title = ""
        status = ""
        for line in text.split("\n"):
            if line.startswith("title:"):
                title = line.split(":", 1)[1].strip().strip('"')
            if line.startswith("status:"):
                status = line.split(":", 1)[1].strip()
        if status_filter and status != status_filter:
            continue
        okrs.append({"file": f.name, "title": title, "status": status})
    return {"success": True, "okrs": okrs, "count": len(okrs)}


def update(args: dict) -> dict:
    _ensure_dir()
    filename = args.get("file", "")
    if not filename:
        return {"success": False, "error": "file is required"}
    filepath = _OKR_DIR / filename
    if not filepath.exists():
        return {"success": False, "error": f"{filename} not found"}

    text = filepath.read_text(encoding="utf-8")
    if args.get("status"):
        text = text.replace(
            f"status: {_extract_field(text, 'status')}",
            f"status: {args['status']}",
        )
    text = text.replace(
        f"updated: {_extract_field(text, 'updated')}",
        f"updated: {time.strftime('%Y-%m-%d')}",
    )
    filepath.write_text(text, encoding="utf-8")
    return {"success": True, "message": f"{filename} updated"}


def _extract_field(text: str, field: str) -> str:
    for line in text.split("\n"):
        if line.startswith(f"{field}:"):
            return line.split(":", 1)[1].strip().strip('"')
    return ""


def run(args: dict) -> dict:
    action = args.pop("action", "list")
    handlers = {"create": create, "list": list_okrs, "update": update}
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
