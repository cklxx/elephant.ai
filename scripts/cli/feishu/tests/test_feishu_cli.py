"""Tests for unified Feishu CLI runtime."""

from __future__ import annotations

import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).resolve().parent.parent.parent.parent))

from cli.feishu import feishu_cli


def test_help_overview_contains_next_steps():
    result = feishu_cli.execute({"command": "help"})
    assert result["success"] is True
    assert result["help_level"] == "overview"
    assert result["next_steps"]


def test_help_action_calendar_create():
    result = feishu_cli.execute(
        {"command": "help", "topic": "action", "module": "calendar", "action_name": "create"}
    )
    assert result["success"] is True
    assert result["module"] == "calendar"
    assert result["action"] == "create"
    assert "title" in result["required"]


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
    assert "open-apis/authen/v1/authorize" in result["url"]
    assert result["state"] == "s1"


def test_auth_tenant_token_delegates_legacy(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setattr(feishu_cli.legacy_lark_auth, "get_lark_tenant_token", lambda **_: ("tenant-token", None))
    result = feishu_cli.execute({"command": "auth", "subcommand": "tenant_token", "args": {}})
    assert result["success"] is True
    assert result["access_token"] == "tenant-token"


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
    assert result["result"]["data"]["ok"] is True


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
    assert result["echo"]["start"] == "2026-03-06"
