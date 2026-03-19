#!/usr/bin/env python3
"""openbench skill — Eval 基准测试。

Runs the agent_eval evaluation suite and reports results.
"""

from __future__ import annotations

import subprocess
import sys
import time
from pathlib import Path

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv
from skill_runner.cli_contract import parse_cli_args, render_result
from skill_runner.openmax_utils import repo_root, run_skill_main

load_repo_dotenv(__file__)


# ---------------------------------------------------------------------------
# Actions
# ---------------------------------------------------------------------------


def run_bench(args: dict) -> dict:
    suite = str(args.get("suite", "quick"))
    timeout = int(args.get("timeout", 300))
    output_dir = args.get("output_dir", "")

    root = repo_root()
    bench_dir = Path(output_dir) if output_dir else root / ".openmax" / "bench"
    bench_dir.mkdir(parents=True, exist_ok=True)

    if suite == "quick":
        cmd = [str(root / "scripts" / "eval-quick.sh")]
    elif suite == "full":
        cmd = ["alex", "dev", "test", "./evaluation/..."]
    else:
        return {"success": False, "error": f"unknown suite: {suite!r}. Available: quick, full"}

    start = time.time()
    try:
        result = subprocess.run(
            cmd,
            cwd=root,
            capture_output=True,
            text=True,
            timeout=timeout,
        )
    except subprocess.TimeoutExpired:
        return {"success": False, "error": f"Eval timed out after {timeout}s"}
    except FileNotFoundError as exc:
        return {"success": False, "error": f"Command not found: {exc}"}

    elapsed = time.time() - start
    success = result.returncode == 0

    ts = time.strftime("%Y%m%d-%H%M%S")
    out_file = bench_dir / f"{suite}-{ts}.txt"
    out_file.write_text(result.stdout + "\n" + result.stderr, encoding="utf-8")

    lines = (result.stdout + result.stderr).splitlines()
    tail = "\n".join(lines[-20:]) if lines else ""

    return {
        "success": success,
        "message": f"Eval '{suite}' {'passed' if success else 'FAILED'} in {elapsed:.1f}s.",
        "suite": suite,
        "return_code": result.returncode,
        "elapsed_s": round(elapsed, 1),
        "output_file": str(out_file),
        "tail": tail,
    }


def list_suites(_args: dict) -> dict:
    root = repo_root()
    datasets_dir = root / "evaluation" / "agent_eval" / "datasets"

    suites = [
        {"name": "quick", "command": "scripts/eval-quick.sh", "description": "Quick regression, ~10 cases"},
        {"name": "full", "command": "alex dev test ./evaluation/...", "description": "Full evaluation suite"},
    ]

    if datasets_dir.exists():
        for d in sorted(datasets_dir.iterdir()):
            if d.is_dir():
                suites.append({
                    "name": d.name,
                    "command": f"alex dev test (dataset: {d.name})",
                    "description": "Dataset-based eval",
                })

    return {"success": True, "suites": suites}


def last_result(args: dict) -> dict:
    root = repo_root()
    bench_dir = Path(args.get("output_dir", root / ".openmax" / "bench"))

    if not bench_dir.exists():
        return {"success": True, "message": "No bench results found.", "result": None}

    files = sorted(bench_dir.glob("*.txt"))
    if not files:
        return {"success": True, "message": "No bench result files found.", "result": None}

    latest = files[-1]
    content = latest.read_text(encoding="utf-8")
    lines = content.splitlines()
    tail = "\n".join(lines[-30:]) if lines else ""

    return {
        "success": True,
        "file": str(latest),
        "tail": tail,
    }


def run(args: dict) -> dict:
    action = args.pop("action", "run")
    handlers = {
        "run": run_bench,
        "list": list_suites,
        "last": last_result,
    }
    handler = handlers.get(action)
    if not handler:
        return {"success": False, "error": f"unknown action: {action!r}. Available: {list(handlers)}"}
    return handler(args)


def main() -> None:
    run_skill_main(run)


if __name__ == "__main__":
    main()
