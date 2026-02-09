"""Tests for config-management skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path

import pytest

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("config_management_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

get_config = _mod.get_config
set_config = _mod.set_config
list_config = _mod.list_config
run = _mod.run


@pytest.fixture(autouse=True)
def _config_path(tmp_path, monkeypatch):
    monkeypatch.setattr(_mod, "_CONFIG_PATH", tmp_path / "config.yaml")


class TestGetConfig:
    def test_empty_config(self):
        result = get_config({})
        assert result["success"] is True
        assert result["config"] == {}

    def test_key_not_found(self):
        result = get_config({"key": "nonexistent"})
        assert result["success"] is False

    def test_gets_value(self):
        set_config({"key": "llm.model", "value": "gpt-4o"})
        result = get_config({"key": "llm.model"})
        assert result["success"] is True
        assert result["value"] == "gpt-4o"


class TestSetConfig:
    def test_missing_key(self):
        result = set_config({})
        assert result["success"] is False

    def test_sets_value(self, tmp_path):
        result = set_config({"key": "llm.model", "value": "gpt-4o"})
        assert result["success"] is True
        config = get_config({})["config"]
        assert config["llm.model"] == "gpt-4o"

    def test_overwrites_value(self):
        set_config({"key": "k", "value": "v1"})
        set_config({"key": "k", "value": "v2"})
        result = get_config({"key": "k"})
        assert result["value"] == "v2"


class TestListConfig:
    def test_empty(self):
        result = list_config({})
        assert result["success"] is True
        assert result["count"] == 0

    def test_lists_all(self):
        set_config({"key": "a", "value": "1"})
        set_config({"key": "b", "value": "2"})
        result = list_config({})
        assert result["count"] == 2


class TestRun:
    def test_default_action_is_list(self):
        result = run({})
        assert result["success"] is True

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
