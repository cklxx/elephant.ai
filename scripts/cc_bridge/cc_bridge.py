#!/usr/bin/env python3
"""Claude Agent SDK bridge sidecar for elephant.ai.

Reads a JSON config from stdin, runs Claude via the Agent SDK with event
filtering hooks, and emits pre-filtered JSONL events on stdout.

Protocol (stdout JSONL):
  {"type":"tool","tool_name":"Write","summary":"file_path=/src/main.go","files":["/src/main.go"],"iter":3}
  {"type":"result","answer":"...","tokens":5000,"cost":0.15,"iters":12,"is_error":false}
  {"type":"error","message":"something went wrong"}
"""

from __future__ import annotations

import argparse
import asyncio
import json
import os
import signal
import sys
from pathlib import Path
from typing import Any, TextIO

from claude_agent_sdk import (
    ClaudeAgentOptions,
    ClaudeSDKClient,
    HookMatcher,
    ResultMessage,
)

# Tools whose invocations are forwarded to the Go host.
_CORE_TOOLS = frozenset({"Write", "Edit", "Bash", "WebSearch", "NotebookEdit"})

# Read-only / internal tools — always suppressed.
_SKIP_TOOLS = frozenset({
    "Read", "Glob", "Grep", "WebFetch",
    "Skill",
    "Task", "TaskCreate", "TaskUpdate", "TaskList", "TaskGet", "TaskStop",
    "TaskOutput",
})

_iteration = 0
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


def _base_name(tool_name: str) -> str:
    """Strip argument hints like 'Bash(git *)' → 'Bash'."""
    return tool_name.split("(")[0].strip()


def _is_core(tool_name: str) -> bool:
    base = _base_name(tool_name)
    if base in _CORE_TOOLS:
        return True
    if base in _SKIP_TOOLS:
        return False
    # Unknown tools are forwarded for safety.
    return True


def _trim_args(tool_name: str, args: dict[str, Any]) -> str:
    base = _base_name(tool_name)
    if base in ("Write", "Edit"):
        return f"file_path={args.get('file_path', '')}"
    if base == "Bash":
        cmd = args.get("command", "")
        return f"command={cmd[:120]}" if len(cmd) > 120 else f"command={cmd}"
    if base == "WebSearch":
        return f"query={args.get('query', '')}"
    if base == "NotebookEdit":
        return f"notebook={args.get('notebook_path', '')}"
    # Unknown tool: key names only.
    keys = list(args.keys())[:3]
    return ", ".join(f"{k}=..." for k in keys)


def _extract_files(args: dict[str, Any]) -> list[str]:
    return [
        args[k]
        for k in ("file_path", "path", "notebook_path")
        if k in args and isinstance(args[k], str) and args[k]
    ]


async def _post_tool_hook(
    input_data: dict[str, Any],
    tool_use_id: str | None,
    context: Any,
) -> dict[str, Any]:
    global _iteration
    _iteration += 1

    name = input_data.get("tool_name", "")
    if not _is_core(name):
        return {}

    args = input_data.get("tool_input", {})
    if not isinstance(args, dict):
        args = {}

    _emit({
        "type": "tool",
        "tool_name": name,
        "summary": _trim_args(name, args),
        "files": _extract_files(args),
        "iter": _iteration,
    })
    return {}


async def main() -> None:
    global _output_sink, _done_file

    parser = argparse.ArgumentParser(description="Claude Agent SDK bridge")
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

    opts = ClaudeAgentOptions(
        model=cfg.get("model") or None,
        max_turns=cfg.get("max_turns") or None,
        max_budget_usd=cfg.get("max_budget_usd") or None,
        cwd=cfg.get("working_dir") or os.getcwd(),
        hooks={
            "PostToolUse": [HookMatcher(hooks=[_post_tool_hook])],
        },
    )

    execution_mode = str(cfg.get("execution_mode", "execute") or "execute").strip().lower()

    # Permission mode.
    mode = cfg.get("mode", "interactive")
    if execution_mode == "plan":
        mode = "autonomous"
    if mode == "autonomous":
        opts.permission_mode = "bypassPermissions"
        allowed = cfg.get("allowed_tools")
        if allowed:
            opts.allowed_tools = allowed
        elif execution_mode == "plan":
            opts.allowed_tools = ["Read", "Glob", "Grep", "WebSearch"]
    else:
        opts.permission_mode = "default"
        perm_cfg = cfg.get("permission_mcp_config")
        if perm_cfg:
            opts.mcp_servers = perm_cfg

    prompt = cfg.get("prompt", "")
    if not prompt:
        _emit({"type": "error", "message": "prompt is required"})
        _write_done_sentinel()
        sys.exit(1)
    if execution_mode == "plan":
        prompt = (
            prompt
            + "\n\n[Plan Mode]\nProvide an implementation plan only. "
            + "Do not modify files or execute destructive operations."
        )

    try:
        async with ClaudeSDKClient(opts) as client:
            await client.query(prompt)
            async for message in client.receive_response():
                if isinstance(message, ResultMessage):
                    usage = getattr(message, "usage", {}) or {}
                    tokens = (
                        usage.get("input_tokens", 0)
                        + usage.get("output_tokens", 0)
                    )
                    _emit({
                        "type": "result",
                        "answer": getattr(message, "result", "") or "",
                        "tokens": tokens,
                        "cost": getattr(message, "total_cost_usd", 0) or 0,
                        "iters": _iteration,
                        "is_error": getattr(message, "is_error", False),
                    })
    except Exception as exc:
        _emit({"type": "error", "message": str(exc)})
        _write_done_sentinel()
        sys.exit(1)

    _write_done_sentinel()


if __name__ == "__main__":
    asyncio.run(main())
