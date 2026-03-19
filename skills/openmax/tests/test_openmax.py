"""Tests for openmax skill."""

from __future__ import annotations

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
_spec = importlib.util.spec_from_file_location("openmax_run", _RUN_PY)
openmax = importlib.util.module_from_spec(_spec)
sys.modules["openmax_run"] = openmax
_spec.loader.exec_module(openmax)


# ---------------------------------------------------------------------------
# dispatch
# ---------------------------------------------------------------------------


class TestDispatch:
    def test_dispatch_requires_tasks(self):
        result = openmax.dispatch({})
        assert not result["success"]
        assert "--tasks is required" in result["error"]

    def test_dispatch_dry_run(self, tmp_path):
        brief_dir = tmp_path / ".openmax" / "briefs"
        brief_dir.mkdir(parents=True)
        (brief_dir / "task1.md").write_text("Do something.", encoding="utf-8")

        worktree_base = tmp_path / ".openmax-worktrees"

        with patch("openmax_run.repo_root", return_value=tmp_path), \
             patch("openmax_run.create_worktree") as mock_wt, \
             patch("openmax_run.launch_worker") as mock_launch:

            mock_wt.return_value = (worktree_base / "openmax_task1", True)

            result = openmax.dispatch({
                "tasks": "task1",
                "dry_run": True,
                "brief_dir": str(brief_dir),
                "worktree_base": str(worktree_base),
            })

        assert result["success"]
        mock_launch.assert_called_once()
        _, _, _, _, dry_run = mock_launch.call_args[0]
        assert dry_run is True

    def test_dispatch_skips_missing_brief(self, tmp_path):
        brief_dir = tmp_path / ".openmax" / "briefs"
        brief_dir.mkdir(parents=True)

        with patch("openmax_run.repo_root", return_value=tmp_path):
            result = openmax.dispatch({
                "tasks": "nonexistent",
                "brief_dir": str(brief_dir),
            })

        assert result["success"]
        assert result["workers"][0]["status"] == "skipped"

    def test_dispatch_rejects_invalid_task_name(self, tmp_path):
        brief_dir = tmp_path / ".openmax" / "briefs"
        brief_dir.mkdir(parents=True)

        with patch("openmax_run.repo_root", return_value=tmp_path):
            result = openmax.dispatch({
                "tasks": "../../etc/evil",
                "brief_dir": str(brief_dir),
            })

        assert result["success"]  # Overall call succeeds...
        assert result["workers"][0]["status"] == "error"  # ...but task is rejected.
        assert "invalid" in result["workers"][0]["reason"].lower()

    def test_dispatch_multiple_tasks(self, tmp_path):
        brief_dir = tmp_path / ".openmax" / "briefs"
        brief_dir.mkdir(parents=True)
        (brief_dir / "task1.md").write_text("Task 1", encoding="utf-8")
        (brief_dir / "task2.md").write_text("Task 2", encoding="utf-8")
        worktree_base = tmp_path / ".openmax-worktrees"

        with patch("openmax_run.repo_root", return_value=tmp_path), \
             patch("openmax_run.create_worktree") as mock_wt, \
             patch("openmax_run.launch_worker", return_value=12345), \
             patch("openmax_run.inject_claude_md"), \
             patch("openmax_run.inject_brief_context"):

            mock_wt.side_effect = [
                (worktree_base / "openmax_task1", True),
                (worktree_base / "openmax_task2", True),
            ]

            result = openmax.dispatch({
                "tasks": "task1,task2",
                "brief_dir": str(brief_dir),
                "worktree_base": str(worktree_base),
            })

        assert result["success"]
        assert len(result["workers"]) == 2
        assert all(w["status"] == "launched" for w in result["workers"])


# ---------------------------------------------------------------------------
# status
# ---------------------------------------------------------------------------


class TestStatus:
    def test_status_no_worktrees(self, tmp_path):
        with patch("openmax_run.repo_root", return_value=tmp_path):
            result = openmax.status({})
        assert result["success"]
        assert result["workers"] == []

    def test_status_detects_done(self, tmp_path):
        worktree_base = tmp_path / ".openmax-worktrees"
        wt = worktree_base / "openmax_mytask"
        wt.mkdir(parents=True)
        (wt / ".git").mkdir()

        report_dir = tmp_path / ".openmax" / "reports"
        report_dir.mkdir(parents=True)
        (report_dir / "mytask.md").write_text("## Status\ndone", encoding="utf-8")

        with patch("openmax_run.repo_root", return_value=tmp_path):
            result = openmax.status({
                "worktree_base": str(worktree_base),
                "report_dir": str(report_dir),
            })

        assert result["success"]
        assert len(result["workers"]) == 1
        w = result["workers"][0]
        assert w["task"] == "mytask"
        assert w["done"] is True
        assert w["state"] == "done"

    def test_status_running_state(self, tmp_path):
        import os
        worktree_base = tmp_path / ".openmax-worktrees"
        wt = worktree_base / "openmax_runtask"
        wt.mkdir(parents=True)
        (wt / ".git").mkdir()

        pid_dir = tmp_path / ".openmax" / "pids"
        pid_dir.mkdir(parents=True)
        (pid_dir / "runtask.pid").write_text(str(os.getpid()), encoding="utf-8")

        report_dir = tmp_path / ".openmax" / "reports"
        report_dir.mkdir(parents=True)

        with patch("openmax_run.repo_root", return_value=tmp_path):
            result = openmax.status({
                "worktree_base": str(worktree_base),
                "report_dir": str(report_dir),
            })

        w = result["workers"][0]
        assert w["state"] == "running"
        assert not w["done"]


# ---------------------------------------------------------------------------
# collect
# ---------------------------------------------------------------------------


class TestCollect:
    def test_collect_no_reports_dir(self, tmp_path):
        with patch("openmax_run.repo_root", return_value=tmp_path):
            result = openmax.collect({})
        assert result["success"]
        assert result["reports"] == []

    def test_collect_returns_reports(self, tmp_path):
        report_dir = tmp_path / ".openmax" / "reports"
        report_dir.mkdir(parents=True)
        (report_dir / "alpha.md").write_text("## Status\ndone", encoding="utf-8")
        (report_dir / "beta.md").write_text("## Status\nerror", encoding="utf-8")

        with patch("openmax_run.repo_root", return_value=tmp_path):
            result = openmax.collect({"report_dir": str(report_dir)})

        assert result["success"]
        assert len(result["reports"]) == 2
        tasks = [r["task"] for r in result["reports"]]
        assert "alpha" in tasks
        assert "beta" in tasks

    def test_collect_task_filter(self, tmp_path):
        report_dir = tmp_path / ".openmax" / "reports"
        report_dir.mkdir(parents=True)
        (report_dir / "alpha.md").write_text("alpha", encoding="utf-8")
        (report_dir / "beta.md").write_text("beta", encoding="utf-8")

        with patch("openmax_run.repo_root", return_value=tmp_path):
            result = openmax.collect({"report_dir": str(report_dir), "task": "alpha"})

        assert len(result["reports"]) == 1
        assert result["reports"][0]["task"] == "alpha"

    def test_collect_synthesize_calls_claude(self, tmp_path):
        report_dir = tmp_path / ".openmax" / "reports"
        report_dir.mkdir(parents=True)
        (report_dir / "task1.md").write_text("## Status\ndone\n## Summary\nFixed bug.", encoding="utf-8")

        mock_result = MagicMock()
        mock_result.returncode = 0
        mock_result.stdout = "Executive summary: Bug fixed."

        with patch("openmax_run.repo_root", return_value=tmp_path), \
             patch("subprocess.run", return_value=mock_result) as mock_run:
            result = openmax.collect({
                "report_dir": str(report_dir),
                "synthesize": True,
            })

        assert result["success"]
        assert result.get("synthesis") == "Executive summary: Bug fixed."
        mock_run.assert_called_once()

    def test_collect_synthesize_handles_claude_missing(self, tmp_path):
        report_dir = tmp_path / ".openmax" / "reports"
        report_dir.mkdir(parents=True)
        (report_dir / "task1.md").write_text("done", encoding="utf-8")

        with patch("openmax_run.repo_root", return_value=tmp_path), \
             patch("subprocess.run", side_effect=FileNotFoundError("claude")):
            result = openmax.collect({
                "report_dir": str(report_dir),
                "synthesize": True,
            })

        assert result["success"]
        assert result.get("synthesis") is None
        assert "synthesis_error" in result


# ---------------------------------------------------------------------------
# run dispatcher
# ---------------------------------------------------------------------------


class TestRunDispatcher:
    def test_unknown_action(self):
        result = openmax.run({"action": "blorp"})
        assert not result["success"]
        assert "unknown action" in result["error"]

    def test_empty_action(self):
        result = openmax.run({"action": "", "tasks": ""})
        assert not result["success"]
