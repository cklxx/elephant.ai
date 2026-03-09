#!/usr/bin/env python3
"""feishu-bitable skill — delegates to feishu-cli with module=bitable."""

from __future__ import annotations

import json
import sys
from pathlib import Path

# Reuse feishu-cli's run module
_SKILL_DIR = Path(__file__).resolve().parents[1] / "feishu-cli"
sys.path.insert(0, str(_SKILL_DIR))

from run import run as feishu_run  # noqa: E402


def run(args: dict) -> dict:
    args.setdefault("action", "tool")
    args.setdefault("module", "bitable")
    return feishu_run(args)


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
    sys.exit(0 if result.get("success", False) else 1)


if __name__ == "__main__":
    main()
