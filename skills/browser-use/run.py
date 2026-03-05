#!/usr/bin/env python3
"""browser-use skill — Playwright MCP Extension Relay 控制用户已登录的浏览器。

每次调用启动一个 playwright-mcp 进程，通过 stdin JSON-RPC 发送工具调用。
单动作直接调用，多步操作用 pipeline 在同一个浏览器会话内批量执行。

actions: navigate, snapshot, click, type, screenshot, tabs, evaluate, run_code, press_key, wait_for, pipeline
"""

from __future__ import annotations

import json
import os
import subprocess
import sys
import threading
import time
from pathlib import Path

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv
from skill_runner.cli_contract import parse_cli_args, render_result

load_repo_dotenv(__file__)

_CALL_TIMEOUT = int(os.environ.get("BROWSER_SKILL_TIMEOUT", "30"))


def _call_mcp_tools(tool_calls: list[tuple[str, dict]], timeout: int = _CALL_TIMEOUT) -> list[dict]:
    """Spawn one playwright-mcp process, send init + N tool calls, return results."""
    token = os.environ.get("ALEX_BROWSER_BRIDGE_TOKEN", "")
    env = {**os.environ}
    if token:
        env["PLAYWRIGHT_MCP_EXTENSION_TOKEN"] = token

    proc = subprocess.Popen(
        ["npx", "-y", "@playwright/mcp@latest", "--extension"],
        stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
        env=env, text=True,
    )

    lines: list[str] = []

    def _reader():
        for line in proc.stdout:
            line = line.strip()
            if line:
                lines.append(line)

    t = threading.Thread(target=_reader, daemon=True)
    t.start()

    init_msg = json.dumps({
        "jsonrpc": "2.0", "id": 0,
        "method": "initialize",
        "params": {
            "protocolVersion": "2024-11-05",
            "capabilities": {},
            "clientInfo": {"name": "browser-use-skill", "version": "1.0"},
        },
    })
    proc.stdin.write(init_msg + "\n")
    proc.stdin.flush()
    time.sleep(2)

    for i, (tool_name, arguments) in enumerate(tool_calls, start=1):
        call_msg = json.dumps({
            "jsonrpc": "2.0", "id": i,
            "method": "tools/call",
            "params": {"name": tool_name, "arguments": arguments},
        })
        proc.stdin.write(call_msg + "\n")
        proc.stdin.flush()
        if i < len(tool_calls):
            time.sleep(3)

    t.join(timeout=timeout + len(tool_calls) * 5)

    try:
        proc.stdin.close()
    except Exception:
        pass
    try:
        proc.terminate()
        proc.wait(timeout=3)
    except Exception:
        proc.kill()

    response_map: dict[int, dict] = {}
    for raw in lines:
        try:
            msg = json.loads(raw)
        except json.JSONDecodeError:
            continue
        msg_id = msg.get("id")
        if msg_id is not None and msg_id >= 1:
            response_map[msg_id] = msg

    results = []
    for i in range(1, len(tool_calls) + 1):
        msg = response_map.get(i)
        if not msg:
            results.append({"success": False, "error": f"no response for call {i}"})
            continue
        if "error" in msg:
            results.append({"success": False, "error": msg["error"].get("message", str(msg["error"]))})
            continue
        content = msg.get("result", {}).get("content", [])
        texts = [c.get("text", "") for c in content if c.get("type") == "text"]
        results.append({"success": True, "output": "\n".join(texts)})

    return results


def _call_single(tool_name: str, arguments: dict) -> dict:
    results = _call_mcp_tools([(tool_name, arguments)])
    return results[0] if results else {"success": False, "error": "no response"}


# ── Actions ──

def navigate(a: dict) -> dict:
    url = a.get("url", "")
    if not url:
        return {"success": False, "error": "url is required"}
    return _call_single("browser_navigate", {"url": url})


def snapshot(a: dict) -> dict:
    return _call_single("browser_snapshot", {})


def click(a: dict) -> dict:
    ref = a.get("ref", "")
    if not ref:
        return {"success": False, "error": "ref is required (from snapshot)"}
    return _call_single("browser_click", {"ref": ref, "element": a.get("element", "")})


def type_text(a: dict) -> dict:
    ref, text = a.get("ref", ""), a.get("text", "")
    if not ref or not text:
        return {"success": False, "error": "ref and text are required"}
    return _call_single("browser_type", {"ref": ref, "text": text, "submit": a.get("submit", False)})


def screenshot(a: dict) -> dict:
    params = {"type": a.get("format", "png")}
    if a.get("filename"):
        params["filename"] = a["filename"]
    if a.get("full_page"):
        params["fullPage"] = True
    return _call_single("browser_take_screenshot", params)


def tabs(a: dict) -> dict:
    params = {"action": a.get("tab_action", "list")}
    if "index" in a:
        params["index"] = a["index"]
    return _call_single("browser_tabs", params)


def evaluate(a: dict) -> dict:
    fn = a.get("function", "")
    if not fn:
        return {"success": False, "error": "function is required"}
    return _call_single("browser_evaluate", {"function": fn})


def run_code(a: dict) -> dict:
    code = a.get("code", "")
    if not code:
        return {"success": False, "error": "code is required"}
    return _call_single("browser_run_code", {"code": code})


def press_key(a: dict) -> dict:
    key = a.get("key", "")
    if not key:
        return {"success": False, "error": "key is required"}
    return _call_single("browser_press_key", {"key": key})


def wait_for(a: dict) -> dict:
    params = {}
    for src, dst in [("time", "time"), ("text", "text"), ("text_gone", "textGone")]:
        if src in a:
            params[dst] = a[src]
    return _call_single("browser_wait_for", params)


def pipeline(a: dict) -> dict:
    """Run multiple actions in one browser session.

    Example: {"action": "pipeline", "steps": [
        {"tool": "browser_navigate", "args": {"url": "https://x.com"}},
        {"tool": "browser_snapshot", "args": {}}
    ]}
    """
    steps = a.get("steps", [])
    if not steps:
        return {"success": False, "error": "steps is required"}
    tool_calls = [(s["tool"], s.get("args", {})) for s in steps]
    results = _call_mcp_tools(tool_calls)
    return {"success": all(r.get("success") for r in results), "results": results}


_ACTIONS = {
    "navigate": navigate, "snapshot": snapshot, "click": click,
    "type": type_text, "screenshot": screenshot, "tabs": tabs,
    "evaluate": evaluate, "run_code": run_code, "press_key": press_key,
    "wait_for": wait_for, "pipeline": pipeline,
}


def run(args: dict) -> dict:
    action = args.pop("action", "snapshot")
    handler = _ACTIONS.get(action)
    if not handler:
        return {"success": False, "error": f"unknown action: {action} (available: {', '.join(_ACTIONS)})"}
    return handler(args)


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
