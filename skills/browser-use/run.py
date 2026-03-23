#!/usr/bin/env python3
"""browser-use skill — @playwright/cli controlling the user's real Chrome via extension.

Uses a persistent daemon session: tabs, cookies, and login state survive across calls.
Each action is a simple CLI invocation — no MCP subprocess management.

actions: open, navigate, snapshot, click, type, screenshot, tabs, tab_select,
         evaluate, run_code, press_key, wait_for, go_back, go_forward, reload,
         fill, hover, select, upload, pdf, console, network, close
"""

from __future__ import annotations

import os
import platform
import subprocess
import sys

from pathlib import Path

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv
from skill_runner.cli_contract import parse_cli_args, render_result

load_repo_dotenv(__file__)

_TIMEOUT = int(os.environ.get("BROWSER_SKILL_TIMEOUT", "30"))
_CLI = "npx"
_CLI_ARGS = ["-y", "@playwright/cli@latest"]
_IS_MACOS = platform.system() == "Darwin"


def _get_frontmost_bundle_id() -> str | None:
    """Get the bundle ID of the current foreground app (macOS only)."""
    if not _IS_MACOS:
        return None
    try:
        result = subprocess.run(
            ["osascript", "-e",
             'tell application "System Events" to return bundle identifier '
             'of first application process whose frontmost is true'],
            capture_output=True, text=True, timeout=3,
        )
        bid = result.stdout.strip()
        return bid if bid else None
    except Exception:
        return None


def _restore_focus(bundle_id: str | None) -> None:
    """Restore foreground focus to the given app (macOS only)."""
    if not bundle_id or not _IS_MACOS:
        return
    try:
        subprocess.run(
            ["osascript", "-e", f'tell application id "{bundle_id}" to activate'],
            capture_output=True, text=True, timeout=3,
        )
    except Exception:
        pass


def _run_cli(args: list[str], timeout: int = _TIMEOUT, preserve_focus: bool = True) -> dict:
    """Run a playwright-cli command and return structured result."""
    saved_focus = _get_frontmost_bundle_id() if preserve_focus else None

    cmd = [_CLI, *_CLI_ARGS, *args]
    try:
        proc = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=timeout,
            env={**os.environ},
        )
    except subprocess.TimeoutExpired:
        _restore_focus(saved_focus)
        return {"success": False, "error": f"timeout after {timeout}s"}
    except FileNotFoundError:
        _restore_focus(saved_focus)
        return {"success": False, "error": "npx not found — is Node.js installed?"}

    _restore_focus(saved_focus)

    stdout = proc.stdout.strip()
    stderr = proc.stderr.strip()

    if proc.returncode != 0:
        error_msg = stderr or stdout or f"exit code {proc.returncode}"
        return {"success": False, "error": error_msg}

    return {"success": True, "output": stdout}


def _ensure_session() -> dict | None:
    """Ensure a browser daemon session is running with extension mode.

    Returns None if session exists, or an error dict if it fails to start.
    """
    result = _run_cli(["list"], timeout=10)
    if result.get("success") and result.get("output", "").strip():
        output = result["output"]
        # If there's a running session, we're good
        if "pid" in output.lower() or "default" in output.lower():
            return None

    # No session — start one via open --extension
    result = _run_cli(["open", "--extension"], timeout=15)
    if not result.get("success"):
        return {"success": False, "error": f"failed to open browser session: {result.get('error', 'unknown')}"}
    return None


# ── Actions ──

def action_open(a: dict) -> dict:
    """Open browser session, optionally navigate to URL."""
    args = ["open", "--extension"]
    url = a.get("url", "")
    if url:
        args.append(url)
    return _run_cli(args, timeout=15)


def action_navigate(a: dict) -> dict:
    url = a.get("url", "")
    if not url:
        return {"success": False, "error": "url is required"}
    err = _ensure_session()
    if err:
        return err
    return _run_cli(["goto", url])


def action_snapshot(a: dict) -> dict:
    err = _ensure_session()
    if err:
        return err
    args = ["snapshot"]
    filename = a.get("filename", "")
    if filename:
        args.extend(["--filename", filename])
    return _run_cli(args)


def action_click(a: dict) -> dict:
    ref = a.get("ref", "")
    if not ref:
        return {"success": False, "error": "ref is required (from snapshot)"}
    args = ["click", ref]
    button = a.get("button", "")
    if button:
        args.append(button)
    modifiers = a.get("modifiers", "")
    if modifiers:
        args.extend(["--modifiers", modifiers])
    return _run_cli(args)


def action_type(a: dict) -> dict:
    text = a.get("text", "")
    if not text:
        return {"success": False, "error": "text is required"}
    args = ["type", text]
    if a.get("submit"):
        args.append("--submit")
    return _run_cli(args)


def action_fill(a: dict) -> dict:
    ref = a.get("ref", "")
    text = a.get("text", "")
    if not ref or not text:
        return {"success": False, "error": "ref and text are required"}
    return _run_cli(["fill", ref, text])


def action_screenshot(a: dict) -> dict:
    err = _ensure_session()
    if err:
        return err
    args = ["screenshot"]
    ref = a.get("ref", "")
    if ref:
        args.append(ref)
    filename = a.get("filename", "")
    if filename:
        args.extend(["--filename", filename])
    if a.get("full_page"):
        args.append("--full-page")
    return _run_cli(args)


def action_tabs(a: dict) -> dict:
    err = _ensure_session()
    if err:
        return err
    return _run_cli(["tab-list"])


def action_tab_select(a: dict) -> dict:
    index = a.get("index")
    if index is None:
        return {"success": False, "error": "index is required"}
    return _run_cli(["tab-select", str(index)])


def action_tab_new(a: dict) -> dict:
    args = ["tab-new"]
    url = a.get("url", "")
    if url:
        args.append(url)
    return _run_cli(args)


def action_tab_close(a: dict) -> dict:
    args = ["tab-close"]
    index = a.get("index")
    if index is not None:
        args.append(str(index))
    return _run_cli(args)


def action_evaluate(a: dict) -> dict:
    fn = a.get("function", "")
    if not fn:
        return {"success": False, "error": "function is required"}
    args = ["eval", fn]
    ref = a.get("ref", "")
    if ref:
        args.append(ref)
    return _run_cli(args)


def action_run_code(a: dict) -> dict:
    code = a.get("code", "")
    if not code:
        return {"success": False, "error": "code is required"}
    return _run_cli(["run-code", code])


def action_press_key(a: dict) -> dict:
    key = a.get("key", "")
    if not key:
        return {"success": False, "error": "key is required"}
    return _run_cli(["press", key])


def action_wait_for(a: dict) -> dict:
    # wait_for doesn't exist as a CLI command — use eval with polling
    text = a.get("text", "")
    time_ms = a.get("time")
    if time_ms:
        return _run_cli(["eval", f"() => new Promise(r => setTimeout(r, {time_ms}))"])
    if text:
        return _run_cli(["eval", f"() => document.body.innerText.includes('{text}')"])
    return {"success": False, "error": "text or time is required"}


def action_hover(a: dict) -> dict:
    ref = a.get("ref", "")
    if not ref:
        return {"success": False, "error": "ref is required"}
    return _run_cli(["hover", ref])


def action_select(a: dict) -> dict:
    ref = a.get("ref", "")
    value = a.get("value", "")
    if not ref or not value:
        return {"success": False, "error": "ref and value are required"}
    return _run_cli(["select", ref, value])


def action_go_back(_a: dict) -> dict:
    return _run_cli(["go-back"])


def action_go_forward(_a: dict) -> dict:
    return _run_cli(["go-forward"])


def action_reload(_a: dict) -> dict:
    return _run_cli(["reload"])


def action_pdf(a: dict) -> dict:
    return _run_cli(["pdf"])


def action_console(a: dict) -> dict:
    args = ["console"]
    level = a.get("min_level", "")
    if level:
        args.append(level)
    return _run_cli(args)


def action_network(_a: dict) -> dict:
    return _run_cli(["network"])


def action_close(_a: dict) -> dict:
    return _run_cli(["close"])


_ACTIONS = {
    "open": action_open,
    "navigate": action_navigate,
    "snapshot": action_snapshot,
    "click": action_click,
    "type": action_type,
    "fill": action_fill,
    "screenshot": action_screenshot,
    "tabs": action_tabs,
    "tab_select": action_tab_select,
    "tab_new": action_tab_new,
    "tab_close": action_tab_close,
    "evaluate": action_evaluate,
    "run_code": action_run_code,
    "press_key": action_press_key,
    "wait_for": action_wait_for,
    "hover": action_hover,
    "select": action_select,
    "go_back": action_go_back,
    "go_forward": action_go_forward,
    "reload": action_reload,
    "pdf": action_pdf,
    "console": action_console,
    "network": action_network,
    "close": action_close,
}


def run(args: dict) -> dict:
    action = args.pop("action", "snapshot")
    handler = _ACTIONS.get(action)
    if not handler:
        return {"success": False, "error": f"unknown action: {action} (available: {', '.join(sorted(_ACTIONS))})"}
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
