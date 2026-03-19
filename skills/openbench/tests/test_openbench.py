"""Tests for openbench skill."""

from __future__ import annotations

import subprocess
import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

import importlib.util

import pytest

_SCRIPTS_DIR = Path(__file__).resolve().parents[3] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

# Load run.py under a unique module name to avoid collision with other skills.
_RUN_PY = Path(__file__).resolve().parents[1] / "run.py"
_spec = importlib.util.spec_from_file_location("openbench_run", _RUN_PY)
openbench = importlib.util.module_from_spec(_spec)
sys.modules["openbench_run"] = openbench
_spec.loader.exec_module(openbench)


class TestRunBench:
    def test_unknown_suite(self, tmp_path):
        with patch("openbench_run.repo_root", return_value=tmp_path):
            result = openbench.run_bench({"suite": "nonexistent"})
        assert not result["success"]
        assert "unknown suite" in result["error"]

    def test_quick_suite_passes(self, tmp_path):
        mock_result = MagicMock()
        mock_result.returncode = 0
        mock_result.stdout = "PASS: all tests"
        mock_result.stderr = ""

        with patch("openbench_run.repo_root", return_value=tmp_path), \
             patch("subprocess.run", return_value=mock_result):
            result = openbench.run_bench({"suite": "quick", "output_dir": str(tmp_path)})

        assert result["success"]
        assert result["return_code"] == 0
        assert "passed" in result["message"]

    def test_full_suite_fails(self, tmp_path):
        mock_result = MagicMock()
        mock_result.returncode = 1
        mock_result.stdout = ""
        mock_result.stderr = "FAIL: test_foo"

        with patch("openbench_run.repo_root", return_value=tmp_path), \
             patch("subprocess.run", return_value=mock_result):
            result = openbench.run_bench({"suite": "full", "output_dir": str(tmp_path)})

        assert not result["success"]
        assert "FAILED" in result["message"]

    def test_timeout_returns_error(self, tmp_path):
        with patch("openbench_run.repo_root", return_value=tmp_path), \
             patch("subprocess.run", side_effect=subprocess.TimeoutExpired("cmd", 1)):
            result = openbench.run_bench({"suite": "quick", "timeout": 1, "output_dir": str(tmp_path)})

        assert not result["success"]
        assert "timed out" in result["error"]

    def test_command_not_found(self, tmp_path):
        with patch("openbench_run.repo_root", return_value=tmp_path), \
             patch("subprocess.run", side_effect=FileNotFoundError("eval-quick.sh")):
            result = openbench.run_bench({"suite": "quick", "output_dir": str(tmp_path)})

        assert not result["success"]
        assert "not found" in result["error"]

    def test_output_file_written(self, tmp_path):
        mock_result = MagicMock()
        mock_result.returncode = 0
        mock_result.stdout = "ok"
        mock_result.stderr = ""

        with patch("openbench_run.repo_root", return_value=tmp_path), \
             patch("subprocess.run", return_value=mock_result):
            result = openbench.run_bench({"suite": "quick", "output_dir": str(tmp_path)})

        out_file = Path(result["output_file"])
        assert out_file.exists()
        assert "ok" in out_file.read_text()


class TestListSuites:
    def test_lists_builtin_suites(self, tmp_path):
        with patch("openbench_run.repo_root", return_value=tmp_path):
            result = openbench.list_suites({})

        assert result["success"]
        names = [s["name"] for s in result["suites"]]
        assert "quick" in names
        assert "full" in names

    def test_includes_dataset_dirs(self, tmp_path):
        datasets_dir = tmp_path / "evaluation" / "agent_eval" / "datasets"
        datasets_dir.mkdir(parents=True)
        (datasets_dir / "my_dataset").mkdir()

        with patch("openbench_run.repo_root", return_value=tmp_path):
            result = openbench.list_suites({})

        names = [s["name"] for s in result["suites"]]
        assert "my_dataset" in names


class TestLastResult:
    def test_no_bench_dir(self, tmp_path):
        with patch("openbench_run.repo_root", return_value=tmp_path):
            result = openbench.last_result({})
        assert result["success"]
        assert result["result"] is None

    def test_no_files(self, tmp_path):
        bench_dir = tmp_path / "bench"
        bench_dir.mkdir()

        with patch("openbench_run.repo_root", return_value=tmp_path):
            result = openbench.last_result({"output_dir": str(bench_dir)})

        assert result["success"]
        assert result["result"] is None

    def test_returns_latest(self, tmp_path):
        bench_dir = tmp_path / "bench"
        bench_dir.mkdir()
        (bench_dir / "quick-20260101-100000.txt").write_text("first", encoding="utf-8")
        (bench_dir / "quick-20260101-200000.txt").write_text("second", encoding="utf-8")

        with patch("openbench_run.repo_root", return_value=tmp_path):
            result = openbench.last_result({"output_dir": str(bench_dir)})

        assert result["success"]
        assert "second" in result["tail"]


class TestRunDispatcher:
    def test_default_action_is_run(self, tmp_path):
        with patch("openbench_run.repo_root", return_value=tmp_path), \
             patch("subprocess.run", side_effect=FileNotFoundError("eval-quick.sh")):
            result = openbench.run({"output_dir": str(tmp_path)})
        assert not result["success"]

    def test_unknown_action(self):
        result = openbench.run({"action": "explode"})
        assert not result["success"]
        assert "unknown action" in result["error"]
