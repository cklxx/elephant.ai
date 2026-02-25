"""Tests for lark-conversation-governor skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("lark_conversation_governor_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

evaluate = _mod.evaluate
compose = _mod.compose
run = _mod.run


class TestEvaluate:
    def test_stop_signal_disables_proactive(self):
        result = evaluate({"text": "请停止提醒我", "proactive": True})
        assert result["success"] is True
        assert result["decision"] == "disable_proactive"
        assert result["should_send"] is False

    def test_quiet_hours_defers(self):
        result = evaluate({"text": "今天同步进展", "proactive": True, "now_hour": 23, "quiet_hours": [22, 8]})
        assert result["success"] is True
        assert result["decision"] == "defer"
        assert result["should_send"] is False


class TestCompose:
    def test_requires_objective(self):
        result = compose({})
        assert result["success"] is False

    def test_builds_message(self):
        result = compose({"objective": "周报", "status": "70%", "next_step": "补风险"})
        assert result["success"] is True
        assert "目标：周报" in result["message"]


class TestRun:
    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
