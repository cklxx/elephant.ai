"""Tests for unified Feishu CLI runtime."""

from __future__ import annotations

import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parent.parent.parent.parent))

from cli.feishu import feishu_cli


def test_help_overview_contains_next_steps_and_contracts():
    result = feishu_cli.execute({"command": "help"})
    assert result["success"] is True
    assert result["help_level"] == "overview"
    assert result["command"] == "help"
    assert result["next_steps"]
    assert result["request_contracts"]["tool"]["shape"]["command"] == "tool"


def test_help_module_sheets_regression_has_action_specs():
    result = feishu_cli.execute({"command": "help", "topic": "module", "module": "sheets"})
    assert result["success"] is True
    actions = {entry["action"] for entry in result["actions"]}
    assert {"create", "get", "list_sheets"}.issubset(actions)


def test_help_action_calendar_create():
    result = feishu_cli.execute(
        {"command": "help", "topic": "action", "module": "calendar", "action_name": "create"}
    )
    assert result["success"] is True
    assert result["module"] == "calendar"
    assert result["action"] == "create"
    assert "title" in result["required"]


def test_help_action_accepts_alias():
    result = feishu_cli.execute(
        {"command": "help", "topic": "action", "module": "okr", "action_name": "batch_get_okrs"}
    )
    assert result["success"] is True
    assert result["action"] == "batch_get"
    assert "batch_get_okrs" in result["aliases"]


def test_help_action_can_use_action_field_alias():
    result = feishu_cli.execute(
        {"command": "help", "topic": "action", "module": "calendar", "action": "create_event"}
    )
    assert result["success"] is True
    assert result["action"] == "create"


def test_auth_oauth_url_requires_allowlist(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setenv("LARK_APP_ID", "cli_test")
    monkeypatch.setenv("LARK_OAUTH_REDIRECT_URI", "https://example.com/callback")
    monkeypatch.delenv("LARK_OAUTH_REDIRECT_ALLOWLIST", raising=False)
    monkeypatch.delenv("FEISHU_OAUTH_REDIRECT_ALLOWLIST", raising=False)

    result = feishu_cli.execute(
        {
            "command": "auth",
            "subcommand": "oauth_url",
            "args": {"scopes": ["contact:user.base:readonly"]},
        }
    )
    assert result["success"] is False
    assert "allowlist" in result["error"]


def test_auth_oauth_url_success(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setenv("LARK_APP_ID", "cli_test")
    monkeypatch.setenv("LARK_OAUTH_REDIRECT_URI", "https://example.com/callback")
    monkeypatch.setenv("LARK_OAUTH_REDIRECT_ALLOWLIST", "https://example.com/callback")

    result = feishu_cli.execute(
        {
            "command": "auth",
            "subcommand": "oauth_url",
            "args": {"scopes": ["contact:user.base:readonly"], "state": "s1"},
        }
    )
    assert result["success"] is True
    assert result["command"] == "auth"
    assert "open-apis/authen/v1/authorize" in result["url"]
    assert result["state"] == "s1"


def test_auth_tenant_token_delegates_legacy(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setattr(feishu_cli.legacy_lark_auth, "get_lark_tenant_token", lambda **_: ("tenant-token", None))
    result = feishu_cli.execute({"command": "auth", "subcommand": "tenant_token", "args": {}})
    assert result["success"] is True
    assert result["command"] == "auth"
    assert result["subcommand"] == "tenant_token"
    assert result["access_token"] == "tenant-token"


def test_auth_tenant_token_accepts_flat_args(monkeypatch: pytest.MonkeyPatch):
    captured: dict[str, bool] = {}

    def _fake_get_token(*, force_refresh: bool = False, timeout: int = 15):
        captured["force_refresh"] = force_refresh
        return "tenant-token", None

    monkeypatch.setattr(feishu_cli.legacy_lark_auth, "get_lark_tenant_token", _fake_get_token)
    result = feishu_cli.execute({"command": "auth", "subcommand": "tenant_token", "force_refresh": True})
    assert result["success"] is True
    assert captured["force_refresh"] is True


def test_api_tenant_delegates_legacy(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setattr(
        feishu_cli.legacy_lark_auth,
        "lark_api_json",
        lambda *_, **__: {"code": 0, "data": {"ok": True}},
    )
    result = feishu_cli.execute(
        {
            "command": "api",
            "method": "GET",
            "path": "/contact/v3/scopes",
        }
    )
    assert result["success"] is True
    assert result["command"] == "api"
    assert result["result"]["data"]["ok"] is True


def test_api_invalid_auth_returns_context():
    result = feishu_cli.execute({"command": "api", "method": "GET", "path": "/contact/v3/scopes", "auth": "bad"})
    assert result["success"] is False
    assert result["command"] == "api"
    assert result["method"] == "GET"


def test_tool_dispatch(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setitem(
        feishu_cli.TOOL_HANDLERS["calendar"],
        "query",
        lambda args, _auth: {"success": True, "echo": args},
    )
    result = feishu_cli.execute(
        {
            "command": "tool",
            "module": "calendar",
            "tool_action": "query",
            "args": {"start": "2026-03-06"},
        }
    )
    assert result["success"] is True
    assert result["command"] == "tool"
    assert result["module"] == "calendar"
    assert result["action"] == "query"
    assert result["echo"]["start"] == "2026-03-06"


def test_tool_infers_command_and_uses_flat_args(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setitem(
        feishu_cli.TOOL_HANDLERS["calendar"],
        "query",
        lambda args, _auth: {"success": True, "echo": args},
    )
    result = feishu_cli.execute({"module": "calendar", "action": "query", "start": "2026-03-06"})
    assert result["success"] is True
    assert result["command"] == "tool"
    assert result["module"] == "calendar"
    assert result["action"] == "query"
    assert result["echo"]["start"] == "2026-03-06"


def test_tool_args_must_be_object():
    result = feishu_cli.execute({"command": "tool", "module": "calendar", "tool_action": "query", "args": []})
    assert result["success"] is False
    assert "args must be an object" in result["error"]


def test_unknown_command_requires_command_field():
    result = feishu_cli.execute({"command": "invalid"})
    assert result["success"] is False
    assert result["command"] == "invalid"


def test_build_request_from_cli_defaults_to_help_overview():
    result = feishu_cli._build_request_from_cli([])
    assert result == {"command": "help", "topic": "overview"}


def test_build_request_from_cli_tool_parses_json_args():
    result = feishu_cli._build_request_from_cli(["tool", "calendar", "query", '{"start":"2026-03-06"}'])
    assert result["command"] == "tool"
    assert result["module"] == "calendar"
    assert result["tool_action"] == "query"
    assert result["args"]["start"] == "2026-03-06"
