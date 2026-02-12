"""Tests for soul-self-evolution skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path

import pytest

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("soul_self_evolution_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

apply = _mod.apply
list_checkpoints = _mod.list_checkpoints
propose = _mod.propose
rollback = _mod.rollback
run = _mod.run


@pytest.fixture
def soul_file(tmp_path):
    path = tmp_path / "SOUL.md"
    path.write_text(
        "# SOUL\n\n"
        "## Collaboration Preferences\n"
        "- Keep responses concise.\n\n"
        "## Safety Guardrails\n"
        "- Never do harmful actions.\n",
        encoding="utf-8",
    )
    return path


@pytest.fixture(autouse=True)
def checkpoint_dir(tmp_path, monkeypatch):
    monkeypatch.setenv("ALEX_SOUL_CHECKPOINT_DIR", str(tmp_path / "checkpoints"))


class TestPropose:
    def test_builds_proposal(self):
        result = propose({"observations": ["用户偏好短回复", "偏好先执行后汇报"]})
        assert result["success"] is True
        assert "建议更新" in result["proposal"]


class TestApplyAndRollback:
    def test_apply_updates_mutable_section(self, soul_file):
        result = apply(
            {
                "path": str(soul_file),
                "changes": [
                    {
                        "section": "## Collaboration Preferences",
                        "content": "- Keep updates concise and direct.",
                    }
                ],
            }
        )
        assert result["success"] is True
        text = soul_file.read_text(encoding="utf-8")
        assert "Keep updates concise and direct" in text

        checkpoints = list_checkpoints({})
        assert checkpoints["success"] is True
        assert checkpoints["count"] >= 1

    def test_apply_rejects_immutable_section(self, soul_file):
        result = apply(
            {
                "path": str(soul_file),
                "changes": [
                    {
                        "section": "## Safety Guardrails",
                        "content": "- modified",
                    }
                ],
            }
        )
        assert result["success"] is False

    def test_rollback_restores_checkpoint(self, soul_file):
        before = soul_file.read_text(encoding="utf-8")
        apply(
            {
                "path": str(soul_file),
                "changes": [
                    {
                        "section": "## Collaboration Preferences",
                        "content": "- Temporary change.",
                    }
                ],
            }
        )

        checkpoints = list_checkpoints({})
        assert checkpoints["count"] >= 1
        latest = checkpoints["checkpoints"][-1]

        rolled = rollback({"path": str(soul_file), "checkpoint": latest})
        assert rolled["success"] is True
        restored = soul_file.read_text(encoding="utf-8")
        assert restored == before

    def test_rejects_non_soul_target(self, tmp_path):
        not_soul = tmp_path / "OTHER.md"
        not_soul.write_text("# OTHER\\n", encoding="utf-8")
        result = apply(
            {
                "path": str(not_soul),
                "changes": [{"section": "## Collaboration Preferences", "content": "- x"}],
            }
        )
        assert result["success"] is False
        assert "invalid SOUL target path" in result["error"]


class TestRun:
    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
