#!/usr/bin/env python3
"""Generic coding CLI bridge.

Runs an arbitrary CLI command with prompt injection and emits SDKEvent JSONL.
This bridge is intentionally conservative: unknown tools are surfaced as text
stream + final result while preserving stderr/error boundaries.
"""

from __future__ import annotations

import argparse
import json
import os
import signal
import subprocess
import sys
from pathlib import Path
from typing import Any, TextIO

_output_sink: TextIO = sys.stdout
_done_file: str | None = None
_shutting_down = False


def _emit(event: dict[str, Any]) -> None:
    _output_sink.write(json.dumps(event, ensure_ascii=False) + "\n")
    _output_sink.flush()


def _write_done() -> None:
    if _done_file:
        Path(_done_file).touch()


def _sigterm_handler(signum: int, frame: Any) -> None:
    global _shutting_down
    if _shutting_down:
        return
    _shutting_down = True
    _emit({"type": "error", "message": "generic bridge terminated by signal"})
    _write_done()
    sys.exit(128 + signum)


def _render_template(template: str, binary: str, prompt: str, working_dir: str) -> str:
    return (
        template.replace("{binary}", binary)
        .replace("{prompt}", prompt)
        .replace("{working_dir}", working_dir)
    )


def main() -> None:
    global _output_sink, _done_file

    parser = argparse.ArgumentParser(description="Generic CLI bridge")
    parser.add_argument("--output-file", help="write JSONL events to file")
    args = parser.parse_args()

    if args.output_file:
        _output_sink = open(args.output_file, "w", buffering=1)
        _done_file = str(Path(args.output_file).parent / ".done")

    signal.signal(signal.SIGTERM, _sigterm_handler)

    raw = sys.stdin.readline()
    if not raw.strip():
        _emit({"type": "error", "message": "empty config on stdin"})
        _write_done()
        sys.exit(1)

    try:
        cfg = json.loads(raw)
    except json.JSONDecodeError as exc:
        _emit({"type": "error", "message": f"invalid config JSON: {exc}"})
        _write_done()
        sys.exit(1)

    binary = str(cfg.get("binary") or "").strip()
    prompt = str(cfg.get("prompt") or "")
    if not binary:
        _emit({"type": "error", "message": "binary is required for generic_cli"})
        _write_done()
        sys.exit(1)
    if not prompt:
        _emit({"type": "error", "message": "prompt is required"})
        _write_done()
        sys.exit(1)

    working_dir = str(cfg.get("working_dir") or os.getcwd())
    execution_mode = str(cfg.get("execution_mode") or "execute").strip().lower()

    execute_tmpl = str(cfg.get("execute_command") or "").strip()
    plan_tmpl = str(cfg.get("plan_command") or "").strip()

    if execution_mode == "plan" and plan_tmpl:
        shell_cmd = _render_template(plan_tmpl, binary, prompt, working_dir)
        cmd = ["sh", "-lc", shell_cmd]
    elif execute_tmpl:
        shell_cmd = _render_template(execute_tmpl, binary, prompt, working_dir)
        cmd = ["sh", "-lc", shell_cmd]
    else:
        # Pragmatic fallback for many single-shot CLIs.
        cmd = [binary, prompt]

    _emit({"type": "tool", "tool_name": "generic_cli.start", "summary": "launch", "files": [], "iter": 1})

    try:
        proc = subprocess.Popen(
            cmd,
            cwd=working_dir,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )
    except FileNotFoundError:
        _emit({"type": "error", "message": f"binary not found: {binary}"})
        _write_done()
        sys.exit(1)
    except Exception as exc:
        _emit({"type": "error", "message": str(exc)})
        _write_done()
        sys.exit(1)

    lines: list[str] = []
    iter_count = 1

    if proc.stdout is not None:
        for line in proc.stdout:
            text = line.rstrip("\n")
            if not text:
                continue
            lines.append(text)
            iter_count += 1
            _emit(
                {
                    "type": "tool",
                    "tool_name": "generic_cli.stdout",
                    "summary": text[:160],
                    "files": [],
                    "iter": iter_count,
                }
            )

    stderr_text = ""
    if proc.stderr is not None:
        stderr_text = proc.stderr.read()[:2000]

    rc = proc.wait()
    answer = "\n".join(lines).strip()

    if rc != 0:
        msg = f"generic_cli exited with code {rc}"
        if stderr_text.strip():
            msg = f"{msg}: {stderr_text.strip()}"
        _emit({"type": "error", "message": msg})
        _write_done()
        sys.exit(1)

    _emit(
        {
            "type": "result",
            "answer": answer,
            "tokens": 0,
            "cost": 0,
            "iters": iter_count,
            "is_error": False,
        }
    )
    _write_done()


if __name__ == "__main__":
    main()
