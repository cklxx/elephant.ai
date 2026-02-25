#!/usr/bin/env python3
"""Codex bridge sidecar for elephant.ai.

Reads a JSON config from stdin, runs `codex exec --json`, translates Codex
JSONL events into the unified SDKEvent format, and emits them on stdout.

Protocol (stdout JSONL) — identical to cc_bridge.py:
  {"type":"tool","tool_name":"Bash","summary":"command=npm test","files":[],"iter":1}
  {"type":"result","answer":"...","tokens":5000,"cost":0,"iters":3,"is_error":false}
  {"type":"error","message":"something went wrong"}

Codex JSONL event types (codex exec --json):
  thread.started   — session metadata
  turn.started     — new LLM turn
  item.started     — command_execution (with command, status=in_progress)
  item.completed   — command_execution, reasoning, agent_message (with text/output)
  turn.completed   — usage stats (input_tokens, output_tokens, cached_input_tokens)
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


def _handle_item_started(
    item: dict[str, Any],
    iteration: int,
) -> int:
    """Process an item.started event. Returns updated iteration count."""
    item_type = item.get("type", "")

    if item_type == "command_execution":
        iteration += 1
        cmd = item.get("command", "")
        # Strip shell wrapper prefix if present (e.g. "/bin/zsh -lc \"...\"")
        if cmd.startswith("/bin/") and " -lc " in cmd:
            # Extract inner command from shell wrapper
            idx = cmd.index(" -lc ") + 5
            inner = cmd[idx:].strip().strip('"')
            cmd = inner
        summary = f"command={cmd[:120]}" if len(cmd) > 120 else f"command={cmd}"
        _emit({
            "type": "tool",
            "tool_name": "Bash",
            "summary": summary,
            "files": [],
            "iter": iteration,
        })

    elif item_type == "mcpToolCall":
        tool_name = item.get("tool", "")
        if not _is_suppressed(tool_name):
            iteration += 1
            _emit({
                "type": "tool",
                "tool_name": tool_name,
                "summary": "",
                "files": [],
                "iter": iteration,
            })

    # reasoning, agent_message started: silently skip
    return iteration


def main() -> None:
    global _output_sink, _done_file

    parser = argparse.ArgumentParser(description="Codex bridge sidecar")
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
    autonomy_level = str(cfg.get("autonomy_level", "controlled") or "controlled").strip().lower()
    if execution_mode == "plan":
        prompt = (
            prompt
            + "\n\n[Plan Mode]\nProvide an implementation plan only. "
            + "Do not modify files or execute destructive operations."
        )

    # Build codex exec command.
    codex_binary = str(cfg.get("binary") or "codex").strip() or "codex"
    cmd = [codex_binary, "exec", "--json"]

    model = cfg.get("model")
    if model:
        cmd.extend(["--model", model])

    # Sandbox mode: read-only | workspace-write | danger-full-access
    sandbox = cfg.get("sandbox")
    if not sandbox and execution_mode == "plan":
        sandbox = "read-only"
    elif not sandbox and autonomy_level == "full":
        sandbox = "danger-full-access"
    if sandbox:
        cmd.extend(["--sandbox", sandbox])

    # Approval policy mapping:
    #   "full-auto"        → --full-auto (sandbox=workspace-write + auto approve)
    #   "dangerously-auto" → --dangerously-bypass-approvals-and-sandbox
    #   other              → ignored (codex uses its default)
    approval = cfg.get("approval_policy", "")
    if not approval and execution_mode == "plan":
        approval = "never"
    elif not approval and autonomy_level == "full":
        approval = "never"
    if approval == "full-auto":
        cmd.append("--full-auto")
    elif approval == "dangerously-auto":
        cmd.append("--dangerously-bypass-approvals-and-sandbox")

    working_dir = cfg.get("working_dir") or os.getcwd()
    cmd.extend(["-C", working_dir, "--skip-git-repo-check"])

    cmd.extend(["--", prompt])

    # Run codex process, read JSONL from stdout.
    iteration = 0
    total_input_tokens = 0
    total_output_tokens = 0
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

            event_type = event.get("type", "")

            if event_type == "item.started":
                item = event.get("item", {})
                iteration = _handle_item_started(item, iteration)

            elif event_type == "item.completed":
                item = event.get("item", {})
                if item.get("type") == "agent_message":
                    text = item.get("text", "")
                    if text:
                        last_answer = text

            elif event_type == "turn.completed":
                usage = event.get("usage", {})
                total_input_tokens += usage.get("input_tokens", 0)
                total_output_tokens += usage.get("output_tokens", 0)

            # thread.started, turn.started, etc: silently skip

        proc.wait()

        if proc.returncode != 0:
            stderr = proc.stderr.read() if proc.stderr else ""  # type: ignore[union-attr]
            _emit({
                "type": "error",
                "message": f"codex exited with code {proc.returncode}: {stderr[:400]}",
            })
            sys.exit(1)

    except FileNotFoundError:
        _emit({"type": "error", "message": f"codex binary not found: {codex_binary}"})
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
        "tokens": total_input_tokens + total_output_tokens,
        "cost": 0,  # Codex does not expose cost
        "iters": iteration,
        "is_error": False,
    })

    _write_done_sentinel()


if __name__ == "__main__":
    main()
