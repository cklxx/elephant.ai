#!/usr/bin/env python3
"""cc-hooks-setup: Configure Claude Code hooks for Lark notification sync.

Usage:
    python3 skills/cc-hooks-setup/run.py '{"action":"setup","server_url":"http://localhost:8080","token":"..."}'
    python3 skills/cc-hooks-setup/run.py '{"action":"remove"}'
"""

import json
import os
import sys
from pathlib import Path


def _settings_path(project_dir: str | None) -> Path:
    base = Path(project_dir) if project_dir else Path.cwd()
    return base / ".claude" / "settings.local.json"


def _build_hook_command(server_url: str, token: str) -> str:
    env_prefix = f"ELEPHANT_HOOKS_URL={server_url}"
    if token:
        env_prefix += f" ELEPHANT_HOOKS_TOKEN={token}"
    return f'{env_prefix} "$CLAUDE_PROJECT_DIR"/scripts/cc_hooks/notify_lark.sh'


def _build_hooks_block(command: str) -> dict:
    hook_entry = {
        "type": "command",
        "command": command,
        "async": True,
        "timeout": 10,
    }
    return {
        "PostToolUse": [{"hooks": [hook_entry]}],
        "Stop": [{"hooks": [hook_entry]}],
    }


def _atomic_write(path: Path, data: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    tmp = path.with_suffix(".tmp")
    tmp.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n")
    tmp.rename(path)


def setup(args: dict) -> dict:
    server_url = args.get("server_url", "").strip()
    if not server_url:
        return {"success": False, "error": "server_url is required"}

    token = args.get("token", "").strip()
    project_dir = args.get("project_dir")

    command = _build_hook_command(server_url, token)
    hooks = _build_hooks_block(command)

    settings_path = _settings_path(project_dir)
    existing: dict = {}
    if settings_path.exists():
        existing = json.loads(settings_path.read_text())

    existing["hooks"] = hooks
    _atomic_write(settings_path, existing)

    return {
        "success": True,
        "path": str(settings_path),
        "message": "Claude Code hooks 配置完成",
    }


def remove(args: dict) -> dict:
    project_dir = args.get("project_dir")
    settings_path = _settings_path(project_dir)

    if not settings_path.exists():
        return {"success": True, "message": "配置文件不存在，无需清理"}

    existing = json.loads(settings_path.read_text())
    if "hooks" not in existing:
        return {"success": True, "message": "配置中没有 hooks，无需清理"}

    del existing["hooks"]

    if existing:
        _atomic_write(settings_path, existing)
    else:
        settings_path.unlink()

    return {"success": True, "message": "Claude Code hooks 已移除"}


def main() -> None:
    if len(sys.argv) < 2:
        print(json.dumps({"success": False, "error": "usage: run.py '{\"action\":\"setup\",...}'"}))
        sys.exit(1)

    args = json.loads(sys.argv[1])
    action = args.get("action", "setup")

    if action == "setup":
        result = setup(args)
    elif action == "remove":
        result = remove(args)
    else:
        result = {"success": False, "error": f"unknown action: {action}"}

    print(json.dumps(result, ensure_ascii=False))
    if not result.get("success"):
        sys.exit(1)


if __name__ == "__main__":
    main()
