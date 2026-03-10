#!/usr/bin/env python3
"""diagram-to-image skill — Mermaid/icon-block → PNG/SVG。

依赖: mmdc (mermaid-cli) 需通过 npm install -g @mermaid-js/mermaid-cli 安装。
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
import subprocess
import tempfile
import time


def render_mermaid(args: dict) -> dict:
    code = args.get("code", "")
    if not code:
        return {"success": False, "error": "code (mermaid source) is required"}

    theme = args.get("theme", "default")
    fmt = args.get("format", "png")
    output = args.get("output", f"/tmp/diagram_{int(time.time())}.{fmt}")

    with tempfile.NamedTemporaryFile(mode="w", suffix=".mmd", delete=False) as f:
        f.write(code)
        input_path = f.name

    try:
        cmd = ["mmdc", "-i", input_path, "-o", output, "-t", theme, "-b", "transparent"]
        if fmt == "svg":
            cmd.extend(["-e", "svg"])
        result = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
        if result.returncode != 0:
            return {"success": False, "error": f"mmdc failed: {result.stderr.strip()}"}
        if not Path(output).exists():
            return {"success": False, "error": "output file not generated"}
        return {"success": True, "path": output, "format": fmt, "message": f"图表已渲染到 {output}"}
    except FileNotFoundError:
        return {"success": False, "error": "mmdc not found. Install: npm install -g @mermaid-js/mermaid-cli"}
    except subprocess.TimeoutExpired:
        return {"success": False, "error": "render timeout (30s)"}
    finally:
        os.unlink(input_path)


def run(args: dict) -> dict:
    action = args.pop("action", "render")
    if action == "render":
        return render_mermaid(args)
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
