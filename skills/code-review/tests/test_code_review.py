"""Tests for code-review skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("code_review_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

collect = _mod.collect
run = _mod.run
_sh = _mod._sh


class TestSh:
    def test_returns_string(self):
        result = _sh("echo hello")
        assert result == "hello"

    def test_returns_empty_on_failure(self):
        result = _sh("false")
        assert result == ""


class TestCollect:
    def test_no_changes(self):
        with patch.object(_mod, "_sh", return_value=""):
            result = collect({})
            assert result["success"] is False
            assert "no changes" in result["error"]

    def test_with_diff(self, tmp_path):
        diff_text = "+++ b/file.py\n+new line"
        call_count = {"n": 0}

        def mock_sh(cmd, cwd=None):
            call_count["n"] += 1
            if "diff" in cmd and "--name-only" in cmd:
                return "file.py"
            if "diff" in cmd and "--stat" in cmd:
                return "1 file changed"
            if "diff" in cmd:
                return diff_text
            if "log" in cmd:
                return "abc1234 fix bug"
            return ""

        (tmp_path / "file.py").write_text("content")
        with patch.object(_mod, "_sh", side_effect=mock_sh):
            result = collect({"cwd": str(tmp_path)})
            assert result["success"] is True
            assert result["diff"] == diff_text
            assert "file.py" in result["changed_files"]
            assert result["file_count"] == 1
            assert "review_prompt" in result
            assert "架构" in result["review_prompt"]

    def test_with_paths_filter(self):
        def mock_sh(cmd, cwd=None):
            if "--name-only" in cmd:
                return "src/main.py"
            if "--stat" in cmd:
                return "1 file"
            if "log" in cmd:
                return ""
            if "diff" in cmd:
                return "+changed" if "src/" in cmd else ""
            return ""

        with patch.object(_mod, "_sh", side_effect=mock_sh):
            result = collect({"paths": ["src/"]})
            assert result["success"] is True

    def test_falls_back_to_cached(self):
        call_count = {"n": 0}

        def mock_sh(cmd, cwd=None):
            call_count["n"] += 1
            if "--cached" in cmd:
                return "+staged change"
            if "--name-only" in cmd:
                return "file.py"
            if "--stat" in cmd:
                return "1 file"
            if "log" in cmd:
                return ""
            if "diff" in cmd and "--cached" not in cmd:
                return ""
            return ""

        with patch.object(_mod, "_sh", side_effect=mock_sh):
            result = collect({})
            assert result["success"] is True

    def test_file_contents_included(self, tmp_path):
        (tmp_path / "small.py").write_text("print('hello')")

        def mock_sh(cmd, cwd=None):
            if "--name-only" in cmd:
                return "small.py"
            if "--stat" in cmd:
                return "1 file"
            if "log" in cmd:
                return ""
            return "+diff"

        with patch.object(_mod, "_sh", side_effect=mock_sh):
            result = collect({"cwd": str(tmp_path)})
            assert result["success"] is True
            assert "small.py" in result["file_contents"]


class TestRun:
    def test_default_action_is_collect(self):
        with patch.object(_mod, "_sh", return_value=""):
            result = run({})
            assert result["success"] is False  # no changes

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
