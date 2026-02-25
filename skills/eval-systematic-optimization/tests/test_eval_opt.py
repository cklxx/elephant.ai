"""Tests for eval-systematic-optimization skill."""

from __future__ import annotations

import importlib.util
import json
from pathlib import Path
from unittest.mock import MagicMock, patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("eval_opt_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

run_baseline = _mod.run_baseline
analyze_failures = _mod.analyze_failures
run = _mod.run


class TestRunBaseline:
    def test_timeout(self):
        import subprocess
        with patch("subprocess.run", side_effect=subprocess.TimeoutExpired("go", 600)):
            result = run_baseline({})
            assert result["success"] is False
            assert "timeout" in result["error"]

    def test_go_not_found(self):
        with patch("subprocess.run", side_effect=FileNotFoundError):
            result = run_baseline({})
            assert result["success"] is False

    def test_success(self):
        mock_result = MagicMock()
        mock_result.returncode = 0
        mock_result.stdout = "pass"
        mock_result.stderr = ""
        with patch("subprocess.run", return_value=mock_result):
            result = run_baseline({})
            assert result["success"] is True
            assert "output_dir" in result


class TestAnalyzeFailures:
    def test_missing_file(self):
        result = analyze_failures({})
        assert result["success"] is False

    def test_file_not_found(self):
        result = analyze_failures({"result_file": "/nonexistent/file.json"})
        assert result["success"] is False

    def test_no_failures(self, tmp_path):
        f = tmp_path / "results.json"
        f.write_text(json.dumps([
            {"name": "case1", "hit_rank": 1, "expected_tools": "tool_a", "top1_tool": "tool_a"},
        ]))
        result = analyze_failures({"result_file": str(f)})
        assert result["success"] is True
        assert result["total_failures"] == 0
        assert len(result["clusters"]) == 0

    def test_with_failures(self, tmp_path):
        f = tmp_path / "results.json"
        f.write_text(json.dumps([
            {"name": "case1", "hit_rank": 3, "expected_tools": "tool_a", "top1_tool": "tool_b", "collection": "col1"},
            {"name": "case2", "hit_rank": 2, "expected_tools": "tool_a", "top1_tool": "tool_b", "collection": "col1"},
            {"name": "case3", "hit_rank": 2, "expected_tools": "tool_c", "top1_tool": "tool_d", "collection": "col2"},
        ]))
        result = analyze_failures({"result_file": str(f)})
        assert result["success"] is True
        assert result["total_failures"] == 3
        assert len(result["clusters"]) == 2
        # Most frequent cluster first
        assert result["clusters"][0]["count"] == 2
        assert "optimization_prompt" in result

    def test_skips_na(self, tmp_path):
        f = tmp_path / "results.json"
        f.write_text(json.dumps([
            {"name": "case1", "hit_rank": 3, "status": "N/A", "expected_tools": "x", "top1_tool": "y"},
        ]))
        result = analyze_failures({"result_file": str(f)})
        assert result["total_failures"] == 0

    def test_nested_cases_key(self, tmp_path):
        f = tmp_path / "results.json"
        f.write_text(json.dumps({
            "cases": [
                {"name": "c1", "hit_rank": 2, "expected_tools": "a", "top1_tool": "b"},
            ]
        }))
        result = analyze_failures({"result_file": str(f)})
        assert result["total_failures"] == 1

    def test_results_key_fallback(self, tmp_path):
        f = tmp_path / "results.json"
        f.write_text(json.dumps({
            "results": [
                {"name": "c1", "hit_rank": 2, "expected_tools": "a", "top1_tool": "b"},
            ]
        }))
        result = analyze_failures({"result_file": str(f)})
        assert result["total_failures"] == 1


class TestRun:
    def test_default_action_is_analyze(self):
        result = run({})
        assert result["success"] is False  # missing result_file

    def test_baseline_action(self):
        with patch("subprocess.run", side_effect=FileNotFoundError):
            result = run({"action": "baseline"})
            assert result["success"] is False

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
