"""End-to-end tests for browser-use skill — real subprocess, no mocks.

Spins up a tiny fake MCP server that speaks JSON-RPC over stdin/stdout,
then drives _call_mcp_tools against it to verify event-based signaling.
"""

from __future__ import annotations

import importlib.util
import json
import subprocess
import sys
import textwrap
import time
from pathlib import Path

import pytest

# ── Import the module under test ──
_RUN_PATH = Path(__file__).resolve().parent / "run.py"
_spec = importlib.util.spec_from_file_location("browser_use_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
sys.modules[_spec.name] = _mod
_spec.loader.exec_module(_mod)

_call_mcp_tools = _mod._call_mcp_tools

# ── Fake MCP server scripts ──

_FAKE_MCP_NORMAL = textwrap.dedent("""\
    import json, sys
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        msg = json.loads(line)
        msg_id = msg["id"]
        if msg["method"] == "initialize":
            resp = {"jsonrpc": "2.0", "id": msg_id, "result": {"capabilities": {}}}
        else:
            name = msg["params"]["name"]
            args = msg["params"].get("arguments", {})
            text = f"executed {name} with {json.dumps(args, sort_keys=True)}"
            resp = {"jsonrpc": "2.0", "id": msg_id, "result": {"content": [{"type": "text", "text": text}]}}
        sys.stdout.write(json.dumps(resp) + "\\n")
        sys.stdout.flush()
""")

_FAKE_MCP_SLOW_INIT = textwrap.dedent("""\
    import json, sys, time
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        msg = json.loads(line)
        msg_id = msg["id"]
        if msg["method"] == "initialize":
            time.sleep(0.5)
            resp = {"jsonrpc": "2.0", "id": msg_id, "result": {"capabilities": {}}}
        else:
            resp = {"jsonrpc": "2.0", "id": msg_id, "result": {"content": [{"type": "text", "text": "ok"}]}}
        sys.stdout.write(json.dumps(resp) + "\\n")
        sys.stdout.flush()
""")

_FAKE_MCP_ERROR = textwrap.dedent("""\
    import json, sys
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        msg = json.loads(line)
        msg_id = msg["id"]
        if msg["method"] == "initialize":
            resp = {"jsonrpc": "2.0", "id": msg_id, "result": {"capabilities": {}}}
        else:
            resp = {"jsonrpc": "2.0", "id": msg_id, "error": {"code": -32000, "message": "tool failed"}}
        sys.stdout.write(json.dumps(resp) + "\\n")
        sys.stdout.flush()
""")

_FAKE_MCP_SILENT = textwrap.dedent("""\
    import json, sys
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        msg = json.loads(line)
        if msg["method"] == "initialize":
            resp = {"jsonrpc": "2.0", "id": msg["id"], "result": {"capabilities": {}}}
            sys.stdout.write(json.dumps(resp) + "\\n")
            sys.stdout.flush()
        # Tool calls: no response (simulate hang)
""")

_FAKE_MCP_MIXED_JSON = textwrap.dedent("""\
    import json, sys
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        msg = json.loads(line)
        msg_id = msg["id"]
        if msg["method"] == "initialize":
            resp = {"jsonrpc": "2.0", "id": msg_id, "result": {"capabilities": {}}}
        else:
            # Emit garbage line first, then valid response
            sys.stdout.write("NOT-JSON garbage\\n")
            sys.stdout.flush()
            resp = {"jsonrpc": "2.0", "id": msg_id, "result": {"content": [{"type": "text", "text": "survived"}]}}
        sys.stdout.write(json.dumps(resp) + "\\n")
        sys.stdout.flush()
""")


def _write_script(tmp_path: Path, name: str, source: str) -> Path:
    p = tmp_path / name
    p.write_text(source, encoding="utf-8")
    return p


def _patch_popen(monkeypatch, script_path: Path):
    """Monkey-patch subprocess.Popen so _call_mcp_tools launches our fake server."""
    import subprocess as sp
    _real_popen = sp.Popen

    class _FakePopen(_real_popen.__class__):
        pass

    def _patched_popen(cmd, **kwargs):
        return _real_popen(
            [sys.executable, str(script_path)],
            stdin=kwargs.get("stdin", sp.PIPE),
            stdout=kwargs.get("stdout", sp.PIPE),
            stderr=kwargs.get("stderr", sp.DEVNULL),
            text=True,
        )

    monkeypatch.setattr(sp, "Popen", _patched_popen)


# ── Tests ──

class TestSingleToolCall:
    """Single tool call — the most common path via _call_single."""

    def test_success(self, tmp_path, monkeypatch):
        script = _write_script(tmp_path, "server.py", _FAKE_MCP_NORMAL)
        _patch_popen(monkeypatch, script)

        results = _call_mcp_tools([("browser_snapshot", {})])
        assert len(results) == 1
        assert results[0]["success"] is True
        assert "browser_snapshot" in results[0]["output"]

    def test_error_response(self, tmp_path, monkeypatch):
        script = _write_script(tmp_path, "server.py", _FAKE_MCP_ERROR)
        _patch_popen(monkeypatch, script)

        results = _call_mcp_tools([("browser_click", {"ref": "1"})])
        assert len(results) == 1
        assert results[0]["success"] is False
        assert "tool failed" in results[0]["error"]


class TestPipeline:
    """Multiple sequential tool calls in one session."""

    def test_three_step_pipeline(self, tmp_path, monkeypatch):
        script = _write_script(tmp_path, "server.py", _FAKE_MCP_NORMAL)
        _patch_popen(monkeypatch, script)

        calls = [
            ("browser_navigate", {"url": "https://example.com"}),
            ("browser_snapshot", {}),
            ("browser_click", {"ref": "42"}),
        ]
        results = _call_mcp_tools(calls)
        assert len(results) == 3
        for i, r in enumerate(results):
            assert r["success"] is True, f"step {i} failed: {r}"
        assert "https://example.com" in results[0]["output"]
        assert "browser_snapshot" in results[1]["output"]
        assert '"ref": "42"' in results[2]["output"]

    def test_ordering_preserved(self, tmp_path, monkeypatch):
        script = _write_script(tmp_path, "server.py", _FAKE_MCP_NORMAL)
        _patch_popen(monkeypatch, script)

        calls = [("tool_a", {"n": 1}), ("tool_b", {"n": 2}), ("tool_c", {"n": 3})]
        results = _call_mcp_tools(calls)
        assert len(results) == 3
        assert "tool_a" in results[0]["output"]
        assert "tool_b" in results[1]["output"]
        assert "tool_c" in results[2]["output"]


class TestEdgeCases:
    """Timeouts, malformed JSON, no response."""

    def test_timeout_returns_error(self, tmp_path, monkeypatch):
        script = _write_script(tmp_path, "server.py", _FAKE_MCP_SILENT)
        _patch_popen(monkeypatch, script)

        start = time.monotonic()
        results = _call_mcp_tools([("browser_snapshot", {})], timeout=2)
        elapsed = time.monotonic() - start

        assert len(results) == 1
        assert results[0]["success"] is False
        assert "no response" in results[0]["error"]
        assert elapsed < 15, "should not hang indefinitely"

    def test_garbage_json_ignored(self, tmp_path, monkeypatch):
        script = _write_script(tmp_path, "server.py", _FAKE_MCP_MIXED_JSON)
        _patch_popen(monkeypatch, script)

        results = _call_mcp_tools([("browser_snapshot", {})])
        assert len(results) == 1
        assert results[0]["success"] is True
        assert "survived" in results[0]["output"]

    def test_slow_init_still_works(self, tmp_path, monkeypatch):
        script = _write_script(tmp_path, "server.py", _FAKE_MCP_SLOW_INIT)
        _patch_popen(monkeypatch, script)

        results = _call_mcp_tools([("browser_snapshot", {})])
        assert len(results) == 1
        assert results[0]["success"] is True


class TestEventSignalingPerformance:
    """Verify event-based approach is faster than sleep-based would be."""

    def test_no_artificial_delay(self, tmp_path, monkeypatch):
        """A fast server should complete in well under the old sleep budget (2s init + 3s/step)."""
        script = _write_script(tmp_path, "server.py", _FAKE_MCP_NORMAL)
        _patch_popen(monkeypatch, script)

        calls = [("tool_a", {}), ("tool_b", {}), ("tool_c", {})]
        start = time.monotonic()
        results = _call_mcp_tools(calls)
        elapsed = time.monotonic() - start

        assert all(r["success"] for r in results)
        # Old sleep-based: 2s + 3s + 3s = 8s minimum. Event-based: < 2s.
        assert elapsed < 3, f"took {elapsed:.1f}s — event signaling may be broken"
