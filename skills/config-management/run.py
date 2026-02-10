#!/usr/bin/env python3
"""config-management skill — 配置管理。

读写 YAML 配置文件（纯 Python YAML subset 解析，无外部依赖）。
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
import re
import sys
from pathlib import Path

_CONFIG_PATH = Path(os.environ.get("ALEX_CONFIG_PATH", os.path.expanduser("~/.alex/config.yaml")))


def _read_config() -> dict:
    if not _CONFIG_PATH.exists():
        return {}
    text = _CONFIG_PATH.read_text(encoding="utf-8")
    # Simple YAML-like parser for flat key: value pairs
    config = {}
    for line in text.split("\n"):
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        match = re.match(r"^([a-zA-Z0-9_.]+)\s*:\s*(.+)$", line)
        if match:
            config[match.group(1)] = match.group(2).strip().strip('"').strip("'")
    return config


def _write_config(config: dict):
    _CONFIG_PATH.parent.mkdir(parents=True, exist_ok=True)
    lines = [f"{k}: {v}" for k, v in sorted(config.items())]
    _CONFIG_PATH.write_text("\n".join(lines) + "\n", encoding="utf-8")


def get_config(args: dict) -> dict:
    key = args.get("key", "")
    config = _read_config()
    if not key:
        return {"success": True, "config": config}
    value = config.get(key)
    if value is None:
        return {"success": False, "error": f"key '{key}' not found"}
    return {"success": True, "key": key, "value": value}


def set_config(args: dict) -> dict:
    key = args.get("key", "")
    value = args.get("value", "")
    if not key:
        return {"success": False, "error": "key is required"}

    config = _read_config()
    config[key] = str(value)
    _write_config(config)
    return {"success": True, "key": key, "value": value, "message": f"配置「{key}」已更新"}


def list_config(args: dict) -> dict:
    config = _read_config()
    return {"success": True, "config": config, "count": len(config)}


def run(args: dict) -> dict:
    action = args.pop("action", "list")
    handlers = {"get": get_config, "set": set_config, "list": list_config}
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
