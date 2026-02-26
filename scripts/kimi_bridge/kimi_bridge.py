#!/usr/bin/env python3
"""Kimi bridge sidecar for elephant.ai.

Reads a JSON config from stdin, runs kimi CLI with `--print --output-format
stream-json`, translates role-based JSONL into the unified SDKEvent format,
and emits them on stdout.

Protocol (stdout JSONL) — identical to codex_bridge.py / cc_bridge.py:
  {"type":"tool","tool_name":"Shell","summary":"command=ls -la","files":[],"iter":1}
  {"type":"result","answer":"...","tokens":0,"cost":0,"iters":3,"is_error":false}
  {"type":"error","message":"something went wrong"}

Kimi v1.7.0 stream-json output (role-based JSONL):
  {"role":"assistant","content":[{"type":"text","text":"..."}],"tool_calls":[...]}
  {"role":"tool","content":"...","tool_call_id":"..."}
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


# Read-only / noisy tools suppressed from progress events.
_SKIP_TOOLS = frozenset({
    "read", "glob", "grep", "webfetch", "web_fetch",
})


_output_sink: TextIO = sys.stdout
_done_file: str | None = None
_shutting_down = False


def _emit(event: dict[str, Any]) -> None:
    """Write a single JSONL event to the output sink (stdout or file)."""
    _output_sink.write(json.dumps(event, ensure_ascii=False) + "\n")
    _output_sink.flush()


def _write_done_sentinel() -> None:
    """Write the .done sentinel file to signal completion."""
    if _done_file:
        Path(_done_file).touch()


def _sigterm_handler(signum: int, frame: Any) -> None:
    """Handle SIGTERM: emit final error event and write .done sentinel."""
    global _shutting_down
    if _shutting_down:
        return
    _shutting_down = True
    try:
        _emit({"type": "error", "message": "bridge terminated by signal"})
        _write_done_sentinel()
    finally:
        sys.exit(128 + signum)


def _is_suppressed(tool_name: str) -> bool:
    return tool_name.lower() in _SKIP_TOOLS


def _build_tool_summary(func_name: str, arguments: dict[str, Any]) -> str:
    """Build a concise summary string for a tool call."""
    name_lower = func_name.lower()
    if name_lower in ("shell", "bash", "execute_command"):
        cmd = str(arguments.get("command", arguments.get("cmd", "")))
        return f"command={cmd[:120]}" if len(cmd) > 120 else f"command={cmd}"
    if not arguments:
        return ""
    first_key = next(iter(arguments))
    first_val = str(arguments[first_key])
    return f"{first_key}={first_val[:100]}" if len(first_val) > 100 else f"{first_key}={first_val}"


def _normalize_tool_name(func_name: str) -> str:
    """Map kimi tool function names to canonical SDKEvent tool names."""
    name_lower = func_name.lower()
    if name_lower in ("shell", "bash", "execute_command", "run_command"):
        return "Shell"
    if name_lower in ("read_file", "readfile"):
        return "read"
    if name_lower in ("write_file", "writefile"):
        return "write"
    if name_lower in ("search", "web_search"):
        return "web_search"
    return func_name


def main() -> None:
    global _output_sink, _done_file

    parser = argparse.ArgumentParser(description="Kimi bridge sidecar")
    parser.add_argument(
        "--output-file",
        help="Write JSONL events to this file instead of stdout",
    )
    args = parser.parse_args()

    # Set up output sink and done sentinel.
    if args.output_file:
        _output_sink = open(args.output_file, "w", buffering=1)
        _done_file = str(Path(args.output_file).parent / ".done")

    # Install SIGTERM handler for graceful shutdown.
    signal.signal(signal.SIGTERM, _sigterm_handler)

    raw = sys.stdin.readline()
    if not raw.strip():
        _emit({"type": "error", "message": "empty config on stdin"})
        _write_done_sentinel()
        sys.exit(1)

    try:
        cfg = json.loads(raw)
    except json.JSONDecodeError as exc:
        _emit({"type": "error", "message": f"invalid config JSON: {exc}"})
        _write_done_sentinel()
        sys.exit(1)

    prompt = cfg.get("prompt", "")
    if not prompt:
        _emit({"type": "error", "message": "prompt is required"})
        _write_done_sentinel()
        sys.exit(1)

    execution_mode = str(cfg.get("execution_mode", "execute") or "execute").strip().lower()
    if execution_mode == "plan":
        prompt = (
            prompt
            + "\n\n[Plan Mode]\nProvide an implementation plan only. "
            + "Do not modify files or execute destructive operations."
        )

    # Build kimi CLI command.
    kimi_binary = str(cfg.get("binary") or "kimi").strip() or "kimi"
    cmd = [kimi_binary, "--print", "--output-format", "stream-json", "--yolo"]

    model = cfg.get("model")
    if model:
        cmd.extend(["--model", model])

    working_dir = cfg.get("working_dir") or os.getcwd()
    cmd.extend(["--work-dir", working_dir])

    max_turns = cfg.get("max_turns")
    if max_turns:
        cmd.extend(["--max-steps-per-turn", str(max_turns)])

    cmd.extend(["-p", prompt])

    # Run kimi process, read stream-json from stdout.
    iteration = 0
    last_answer = ""

    try:
        proc = subprocess.Popen(
            cmd,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )

        for line in proc.stdout:  # type: ignore[union-attr]
            line = line.strip()
            if not line:
                continue
            try:
                event = json.loads(line)
            except json.JSONDecodeError:
                continue

            role = event.get("role", "")

            if role == "assistant":
                # Process tool_calls → emit SDKEvent tool events.
                tool_calls = event.get("tool_calls") or []
                for tc in tool_calls:
                    func_info = tc.get("function", {})
                    raw_name = func_info.get("name", "")
                    tool_name = _normalize_tool_name(raw_name)

                    if _is_suppressed(tool_name):
                        continue

                    iteration += 1
                    try:
                        arguments = json.loads(func_info.get("arguments", "{}"))
                    except (json.JSONDecodeError, TypeError):
                        arguments = {}

                    summary = _build_tool_summary(raw_name, arguments)
                    _emit({
                        "type": "tool",
                        "tool_name": tool_name,
                        "summary": summary,
                        "files": [],
                        "iter": iteration,
                    })

                # Process content blocks → update last_answer (skip think blocks).
                content_blocks = event.get("content") or []
                for block in content_blocks:
                    if isinstance(block, dict):
                        if block.get("type") == "thinking":
                            continue
                        text = block.get("text", "")
                    elif isinstance(block, str):
                        text = block
                    else:
                        continue
                    if text:
                        last_answer = text

            # role=="tool" → tool return, skip (not an SDKEvent)

        proc.wait()

        if proc.returncode != 0:
            stderr = proc.stderr.read() if proc.stderr else ""  # type: ignore[union-attr]
            _emit({
                "type": "error",
                "message": f"kimi exited with code {proc.returncode}: {stderr[:400]}",
            })
            _write_done_sentinel()
            sys.exit(1)

    except FileNotFoundError:
        _emit({"type": "error", "message": f"kimi binary not found: {kimi_binary}"})
        _write_done_sentinel()
        sys.exit(1)
    except Exception as exc:
        _emit({"type": "error", "message": str(exc)})
        _write_done_sentinel()
        sys.exit(1)

    # Emit final result.
    _emit({
        "type": "result",
        "answer": last_answer,
        "tokens": 0,  # kimi stream-json does not expose token usage
        "cost": 0,
        "iters": iteration,
        "is_error": False,
    })

    _write_done_sentinel()


if __name__ == "__main__":
    main()
