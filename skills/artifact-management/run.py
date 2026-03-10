#!/usr/bin/env python3
"""artifact-management skill — 持久化工件管理。

创建/查询/删除报告、文档、证据等文件工件。
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

import os
import time

_ARTIFACTS_DIR = Path(os.environ.get("ALEX_ARTIFACTS_DIR", os.path.expanduser("~/.alex/artifacts")))


def _ensure_dir():
    _ARTIFACTS_DIR.mkdir(parents=True, exist_ok=True)


def create(args: dict) -> dict:
    name = args.get("name", "")
    content = args.get("content", "")
    if not name:
        return {"success": False, "error": "name is required"}

    _ensure_dir()
    filepath = _ARTIFACTS_DIR / name
    filepath.parent.mkdir(parents=True, exist_ok=True)
    filepath.write_text(content, encoding="utf-8")

    return {
        "success": True,
        "path": str(filepath),
        "size": len(content),
        "message": f"工件「{name}」已保存",
    }


def list_artifacts(_args: dict) -> dict:
    _ensure_dir()
    artifacts = []
    for f in sorted(_ARTIFACTS_DIR.rglob("*")):
        if f.is_file():
            artifacts.append({
                "name": str(f.relative_to(_ARTIFACTS_DIR)),
                "size": f.stat().st_size,
                "modified": time.strftime("%Y-%m-%d %H:%M", time.localtime(f.stat().st_mtime)),
            })
    return {"success": True, "artifacts": artifacts, "count": len(artifacts)}


def read(args: dict) -> dict:
    name = args.get("name", "")
    if not name:
        return {"success": False, "error": "name is required"}
    filepath = _ARTIFACTS_DIR / name
    if not filepath.exists():
        return {"success": False, "error": f"artifact '{name}' not found"}
    content = filepath.read_text(encoding="utf-8", errors="replace")
    return {"success": True, "name": name, "content": content[:50000]}


def delete(args: dict) -> dict:
    name = args.get("name", "")
    if not name:
        return {"success": False, "error": "name is required"}
    filepath = _ARTIFACTS_DIR / name
    if not filepath.exists():
        return {"success": False, "error": f"artifact '{name}' not found"}
    filepath.unlink()
    return {"success": True, "message": f"工件「{name}」已删除"}


def run(args: dict) -> dict:
    action = args.pop("action", "list")
    handlers = {"create": create, "list": list_artifacts, "read": read, "delete": delete}
    handler = handlers.get(action)
    if not handler:
        return {"success": False, "error": f"unknown action: {action}"}
    return handler(args)


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
