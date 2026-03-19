"""Tests for openseed skill."""

from __future__ import annotations

import sys
from pathlib import Path
from unittest.mock import patch

import importlib.util

import pytest

_SCRIPTS_DIR = Path(__file__).resolve().parents[3] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

# Load run.py under a unique module name to avoid collision with other skills.
_RUN_PY = Path(__file__).resolve().parents[1] / "run.py"
_spec = importlib.util.spec_from_file_location("openseed_run", _RUN_PY)
openseed = importlib.util.module_from_spec(_spec)
sys.modules["openseed_run"] = openseed
_spec.loader.exec_module(openseed)


class TestSeed:
    def test_requires_task(self):
        result = openseed.seed({})
        assert not result["success"]
        assert "--task is required" in result["error"]

    def test_requires_brief(self):
        result = openseed.seed({"task": "mytask"})
        assert not result["success"]
        assert "brief" in result["error"]

    def test_rejects_invalid_task_name(self):
        result = openseed.seed({"task": "../evil", "brief": "Do something"})
        assert not result["success"]
        assert "invalid" in result["error"].lower()

    def test_dry_run(self, tmp_path):
        with patch("openseed_run.repo_root", return_value=tmp_path):
            result = openseed.seed({
                "task": "mytask",
                "brief": "Do something.",
                "dry_run": True,
            })
        assert result["success"]
        assert "dry" in result["message"].lower()
        assert "plan" in result
        assert result["plan"]["branch"] == "openmax/mytask"

    def test_brief_file_not_found(self, tmp_path):
        with patch("openseed_run.repo_root", return_value=tmp_path):
            result = openseed.seed({
                "task": "mytask",
                "brief_file": str(tmp_path / "nonexistent.md"),
            })
        assert not result["success"]
        assert "not found" in result["error"]

    def test_brief_from_file(self, tmp_path):
        brief_path = tmp_path / "my_brief.md"
        brief_path.write_text("Do something important.", encoding="utf-8")

        with patch("openseed_run.repo_root", return_value=tmp_path), \
             patch("subprocess.run") as mock_run, \
             patch("openseed_run.launch_worker", return_value=99999), \
             patch("openseed_run.inject_claude_md"), \
             patch("openseed_run.inject_brief_context"):

            result = openseed.seed({
                "task": "mytask",
                "brief_file": str(brief_path),
                "dry_run": False,
            })

        assert result["success"]
        assert result["pid"] == 99999

    def test_worktree_already_exists(self, tmp_path):
        wt = tmp_path / ".openmax-worktrees" / "openmax_mytask"
        wt.mkdir(parents=True)
        (wt / ".git").mkdir()

        brief_file = tmp_path / ".openmax" / "briefs" / "mytask.md"
        brief_file.parent.mkdir(parents=True)

        with patch("openseed_run.repo_root", return_value=tmp_path), \
             patch("subprocess.run"), \
             patch("openseed_run.launch_worker", return_value=11111), \
             patch("openseed_run.inject_claude_md"), \
             patch("openseed_run.inject_brief_context"):

            result = openseed.seed({"task": "mytask", "brief": "Do it."})

        assert result["success"]
        assert not result["created_worktree"]

    def test_git_worktree_failure(self, tmp_path):
        import subprocess as sp

        with patch("openseed_run.repo_root", return_value=tmp_path), \
             patch("subprocess.run", side_effect=sp.CalledProcessError(128, "git", stderr="already exists")):

            result = openseed.seed({"task": "newtask", "brief": "Do something."})

        assert not result["success"]
        assert "git worktree add failed" in result["error"]


class TestRunDispatcher:
    def test_default_action_is_seed(self, tmp_path):
        result = openseed.run({"task": ""})
        assert not result["success"]
        assert "--task is required" in result["error"]

    def test_unknown_action(self):
        result = openseed.run({"action": "explode"})
        assert not result["success"]
