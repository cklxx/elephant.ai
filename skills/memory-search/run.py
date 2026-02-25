#!/usr/bin/env python3
"""memory-search skill — 对话记忆检索。

搜索 Markdown 记忆文件，返回匹配结果。
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
import subprocess
import sys
from pathlib import Path

_MEMORY_DIR = Path(os.environ.get("ALEX_MEMORY_DIR", os.path.expanduser("~/.alex/memory")))


def search(args: dict) -> dict:
    query = args.get("query", "")
    if not query:
        return {"success": False, "error": "query is required"}

    if not _MEMORY_DIR.exists():
        return {"success": True, "results": [], "count": 0}

    try:
        result = subprocess.run(
            ["grep", "-rl", "-i", query, str(_MEMORY_DIR)],
            capture_output=True, text=True, timeout=10,
        )
    except (subprocess.TimeoutExpired, FileNotFoundError):
        return {"success": True, "results": [], "count": 0}

    files = [f for f in result.stdout.strip().split("\n") if f]
    results = []
    for f in files[:10]:
        path = Path(f)
        content = path.read_text(encoding="utf-8", errors="replace")
        # Extract matching lines
        matches = [line for line in content.split("\n") if query.lower() in line.lower()]
        results.append({
            "file": path.name,
            "path": str(path),
            "matches": matches[:5],
            "preview": content[:500],
        })

    return {"success": True, "results": results, "count": len(results)}


def get(args: dict) -> dict:
    filename = args.get("file", "")
    if not filename:
        return {"success": False, "error": "file is required"}

    filepath = _MEMORY_DIR / filename
    if not filepath.exists():
        return {"success": False, "error": f"memory '{filename}' not found"}

    content = filepath.read_text(encoding="utf-8", errors="replace")
    return {"success": True, "file": filename, "content": content[:50000]}


def list_memories(args: dict) -> dict:
    if not _MEMORY_DIR.exists():
        return {"success": True, "memories": [], "count": 0}
    memories = []
    for f in sorted(_MEMORY_DIR.rglob("*.md"), key=lambda p: p.stat().st_mtime, reverse=True):
        memories.append({
            "file": f.name,
            "size": f.stat().st_size,
        })
    return {"success": True, "memories": memories[:50], "count": len(memories)}


def run(args: dict) -> dict:
    action = args.pop("action", "search")
    handlers = {"search": search, "get": get, "list": list_memories}
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
