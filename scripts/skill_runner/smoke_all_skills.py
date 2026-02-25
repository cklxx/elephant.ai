#!/usr/bin/env python3
"""Smoke-test all in-repo Python skills with a unified runner.

Default checks:
- discover `skills/*/run.py`
- execute each skill with JSON payload `{"action":"__smoke__"}`
- assert stdout is valid JSON object containing `success` boolean

By default, a non-zero exit code is allowed if JSON contract is valid because
many skills intentionally return structured validation errors for empty input.
Use `--strict-exit` to require exit code 0 for every skill.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path


@dataclass
class SmokeResult:
    skill: str
    returncode: int
    contract_ok: bool
    success_value: bool | None
    error: str = ""


def _discover_run_scripts(skills_root: Path) -> list[Path]:
    return sorted(path for path in skills_root.glob("*/run.py") if path.is_file())


def _run_one(python_bin: str, run_py: Path, payload: str, timeout: int) -> SmokeResult:
    skill = run_py.parent.name
    try:
        completed = subprocess.run(
            [python_bin, str(run_py), payload],
            capture_output=True,
            text=True,
            timeout=timeout,
        )
    except subprocess.TimeoutExpired:
        return SmokeResult(
            skill=skill,
            returncode=124,
            contract_ok=False,
            success_value=None,
            error=f"timed out after {timeout}s",
        )

    stdout = (completed.stdout or "").strip()
    try:
        data = json.loads(stdout)
    except json.JSONDecodeError as exc:
        return SmokeResult(
            skill=skill,
            returncode=completed.returncode,
            contract_ok=False,
            success_value=None,
            error=f"invalid JSON output: {exc}",
        )

    if not isinstance(data, dict) or not isinstance(data.get("success"), bool):
        return SmokeResult(
            skill=skill,
            returncode=completed.returncode,
            contract_ok=False,
            success_value=None,
            error="output JSON must be an object with boolean `success`",
        )

    return SmokeResult(
        skill=skill,
        returncode=completed.returncode,
        contract_ok=True,
        success_value=bool(data["success"]),
    )


def _run_pytest(repo_root: Path, python_bin: str) -> tuple[int, str]:
    completed = subprocess.run(
        [python_bin, "-m", "pytest", "-q", "skills"],
        cwd=repo_root,
        capture_output=True,
        text=True,
    )
    combined = "\n".join(x for x in [(completed.stdout or "").strip(), (completed.stderr or "").strip()] if x)
    return completed.returncode, combined


def _parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Smoke-test all Python skills in the repo.")
    parser.add_argument(
        "--repo-root",
        type=Path,
        default=Path(__file__).resolve().parents[2],
        help="Repository root path (default: auto-detected from this script).",
    )
    parser.add_argument(
        "--python",
        default=sys.executable,
        help="Python interpreter to use (default: current interpreter).",
    )
    parser.add_argument(
        "--payload",
        default='{"action":"__smoke__"}',
        help='JSON payload passed to each run.py (default: \'{"action":"__smoke__"}\').',
    )
    parser.add_argument(
        "--timeout",
        type=int,
        default=20,
        help="Per-skill timeout in seconds (default: 20).",
    )
    parser.add_argument(
        "--strict-exit",
        action="store_true",
        help="Require each skill process to exit with code 0.",
    )
    parser.add_argument(
        "--with-pytest",
        action="store_true",
        help="Additionally run `pytest -q skills` after smoke checks.",
    )
    return parser.parse_args()


def main() -> int:
    args = _parse_args()
    repo_root = args.repo_root.resolve()
    skills_root = repo_root / "skills"

    try:
        json.loads(args.payload)
    except json.JSONDecodeError as exc:
        print(f"Invalid --payload JSON: {exc}", file=sys.stderr)
        return 2

    run_scripts = _discover_run_scripts(skills_root)
    if not run_scripts:
        print(f"No run.py scripts found under {skills_root}", file=sys.stderr)
        return 2

    print(f"Discovered {len(run_scripts)} skills with run.py under {skills_root}.")

    failures: list[SmokeResult] = []
    for run_py in run_scripts:
        result = _run_one(args.python, run_py, args.payload, args.timeout)
        strict_exit_failed = args.strict_exit and result.returncode != 0
        if not result.contract_ok or strict_exit_failed:
            failures.append(result)
        status = "PASS" if result.contract_ok and not strict_exit_failed else "FAIL"
        print(
            f"[{status}] {result.skill:32s} rc={result.returncode:3d} "
            f"contract_ok={str(result.contract_ok):5s} success={result.success_value}"
        )

    if failures:
        print("\nFailures:", file=sys.stderr)
        for item in failures:
            print(
                f"- {item.skill}: rc={item.returncode}, error={item.error or 'strict-exit check failed'}",
                file=sys.stderr,
            )

    pytest_rc = 0
    if args.with_pytest:
        print("\nRunning pytest suite: pytest -q skills")
        pytest_rc, pytest_output = _run_pytest(repo_root, args.python)
        if pytest_output:
            print(pytest_output)
        if pytest_rc != 0:
            print("pytest skills failed", file=sys.stderr)

    pytest_state = "skipped"
    if args.with_pytest:
        pytest_state = "passed" if pytest_rc == 0 else "failed"

    overall_ok = not failures and (pytest_rc == 0 if args.with_pytest else True)
    print(
        f"\nSummary: skills_checked={len(run_scripts)} failures={len(failures)} "
        f"pytest={pytest_state}"
    )
    return 0 if overall_ok else 1


if __name__ == "__main__":
    raise SystemExit(main())
