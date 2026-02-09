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

import json
import os
import subprocess
import sys
from typing import Any


# Read-only / noisy tools suppressed from progress events.
_SKIP_TOOLS = frozenset({
    "read", "glob", "grep", "webfetch", "web_fetch",
})


def _emit(event: dict[str, Any]) -> None:
    """Write a single JSONL event to stdout."""
    sys.stdout.write(json.dumps(event, ensure_ascii=False) + "\n")
    sys.stdout.flush()


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
    raw = sys.stdin.readline()
    if not raw.strip():
        _emit({"type": "error", "message": "empty config on stdin"})
        sys.exit(1)

    try:
        cfg = json.loads(raw)
    except json.JSONDecodeError as exc:
        _emit({"type": "error", "message": f"invalid config JSON: {exc}"})
        sys.exit(1)

    prompt = cfg.get("prompt", "")
    if not prompt:
        _emit({"type": "error", "message": "prompt is required"})
        sys.exit(1)

    # Build codex exec command.
    cmd = ["codex", "exec", "--json"]

    model = cfg.get("model")
    if model:
        cmd.extend(["--model", model])

    # Sandbox mode: read-only | workspace-write | danger-full-access
    sandbox = cfg.get("sandbox")
    if sandbox:
        cmd.extend(["--sandbox", sandbox])

    # Approval policy mapping:
    #   "full-auto"        → --full-auto (sandbox=workspace-write + auto approve)
    #   "dangerously-auto" → --dangerously-bypass-approvals-and-sandbox
    #   other              → ignored (codex uses its default)
    approval = cfg.get("approval_policy", "")
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
        _emit({"type": "error", "message": "codex binary not found in PATH"})
        sys.exit(1)
    except Exception as exc:
        _emit({"type": "error", "message": str(exc)})
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


if __name__ == "__main__":
    main()
