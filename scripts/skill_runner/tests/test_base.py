"""Tests for the skill_runner base module."""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path
from unittest.mock import patch

import pytest

# Ensure scripts/ is on path
sys.path.insert(0, str(Path(__file__).resolve().parent.parent.parent))

from skill_runner.base import Skill, SkillResult


# ── SkillResult tests ────────────────────────────────────────────


class TestSkillResult:
    def test_success_result(self):
        r = SkillResult(success=True, data={"key": "value"}, message="done")
        d = r.to_dict()
        assert d["success"] is True
        assert d["data"] == {"key": "value"}
        assert d["message"] == "done"
        assert "error" not in d

    def test_failure_result(self):
        r = SkillResult(success=False, error="something broke")
        d = r.to_dict()
        assert d["success"] is False
        assert d["error"] == "something broke"
        assert "data" not in d
        assert "message" not in d

    def test_empty_result(self):
        r = SkillResult()
        d = r.to_dict()
        assert d == {"success": True}


# ── Skill helpers tests ──────────────────────────────────────────


class TestSkillHelpers:
    def test_ok(self):
        r = Skill.ok(data=[1, 2, 3], message="items")
        assert r.success is True
        assert r.data == [1, 2, 3]

    def test_fail(self):
        r = Skill.fail("oops")
        assert r.success is False
        assert r.error == "oops"

    def test_sh_success(self):
        out = Skill.sh("echo hello")
        assert out == "hello"

    def test_sh_failure(self):
        with pytest.raises(RuntimeError, match="command failed"):
            Skill.sh("exit 1")

    def test_read_write_file(self, tmp_path):
        p = str(tmp_path / "test.txt")
        Skill.write_file(p, "hello world")
        assert Skill.read_file(p) == "hello world"


# ── Skill.run() integration test ─────────────────────────────────


class EchoSkill(Skill):
    name = "echo"

    def execute(self, action: str, **kwargs) -> SkillResult:
        if action == "fail":
            raise ValueError("intentional error")
        return self.ok(data={"action": action, **kwargs})


class TestSkillRun:
    def test_parse_cli_arg(self):
        with patch.object(sys, "argv", ["run.py", '{"action":"greet","name":"cklxx"}']):
            with pytest.raises(SystemExit) as exc_info:
                EchoSkill.run()
            assert exc_info.value.code == 0

    def test_parse_empty_args(self):
        with patch.object(sys, "argv", ["run.py"]):
            with patch.object(sys.stdin, "isatty", return_value=True):
                with pytest.raises(SystemExit) as exc_info:
                    EchoSkill.run()
                assert exc_info.value.code == 0

    def test_execute_error_returns_failure(self):
        with patch.object(sys, "argv", ["run.py", '{"action":"fail"}']):
            with pytest.raises(SystemExit) as exc_info:
                EchoSkill.run()
            assert exc_info.value.code == 1

    def test_invalid_json_exits_with_error(self):
        with patch.object(sys, "argv", ["run.py", "not-json"]):
            with pytest.raises(SystemExit) as exc_info:
                EchoSkill.run()
            assert exc_info.value.code == 1
