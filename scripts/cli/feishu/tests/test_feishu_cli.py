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


def test_doc_read_includes_urls(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setattr(
        feishu_cli,
        "api_request",
        lambda *_, **__: {"data": {"document": {"document_id": "doccnxxxx", "title": "T"}}},
    )
    result = feishu_cli.execute(
        {
            "command": "tool",
            "module": "doc",
            "tool_action": "read",
            "args": {"document_id": "doccnxxxx"},
        }
    )
    assert result["success"] is True
    assert result["document"]["url"] == "https://feishu.cn/docx/doccnxxxx"
    assert result["document"]["applink_url"] == "https://applink.feishu.cn/client/docs/doccnxxxx"


def test_doc_create_accepts_markdown_alias(monkeypatch: pytest.MonkeyPatch):
    calls: list[tuple[str, str, dict | None]] = []

    def fake_api_request(method, path, body=None, **kwargs):
        calls.append((method, path, body))
        if path == "/docx/v1/documents":
            return {"data": {"document": {"document_id": "doccncreate", "title": "Alias"}}}
        raise AssertionError(f"unexpected path: {path}")

    monkeypatch.setattr(feishu_cli, "api_request", fake_api_request)
    monkeypatch.setattr(
        feishu_cli,
        "_doc_write_markdown",
        lambda args, _auth: {"success": True, "echo": args},
    )
    monkeypatch.setattr(
        feishu_cli,
        "_ensure_default_doc_manager_permission",
        lambda *_: {"success": True, "member_id": "ou_xxx", "perm": "full_access", "source": "env"},
    )

    result = feishu_cli.execute(
        {
            "command": "tool",
            "module": "doc",
            "tool_action": "create",
            "args": {"title": "Alias", "markdown": "# hello"},
        }
    )
    assert result["success"] is True
    assert result["document"]["url"] == "https://feishu.cn/docx/doccncreate"
    assert result["content_written"] is True
    assert result["default_manager_permission"]["perm"] == "full_access"


def test_resolve_default_doc_manager_open_id_from_env(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setenv("FEISHU_DOC_DEFAULT_MANAGER_OPEN_ID", "ou_env")
    monkeypatch.delenv("LARK_DOC_DEFAULT_MANAGER_OPEN_ID", raising=False)
    assert feishu_cli._resolve_default_doc_manager_open_id() == ("ou_env", "env")


def test_ensure_default_doc_manager_permission_updates_existing_member(monkeypatch: pytest.MonkeyPatch):
    calls: list[tuple[str, str, dict | None, object]] = []

    def fake_api_request(method, path, body=None, *, query=None, **kwargs):
        calls.append((method, path, body, query))
        if method == "GET" and path == "/drive/v1/permissions/doccncreate/members":
            return {
                "code": 0,
                "data": {"items": [{"member_id": "ou_env", "member_type": "openid", "perm": "view"}]},
            }
        if method == "PUT" and path == "/drive/v1/permissions/doccncreate/members/ou_env":
            return {"code": 0, "data": {"member": {"member_id": "ou_env", "perm": "full_access"}}}
        raise AssertionError(f"unexpected call: {(method, path, body, query)}")

    monkeypatch.setattr(feishu_cli, "api_request", fake_api_request)
    monkeypatch.setattr(feishu_cli, "_resolve_default_doc_manager_open_id", lambda: ("ou_env", "env"))

    result = feishu_cli._ensure_default_doc_manager_permission("doccncreate", auth_manager=None)
    assert result["success"] is True
    assert result["perm"] == "full_access"
    assert any(call[0] == "PUT" for call in calls)


# ---- Domain subcommand tests ----


def test_domain_communicate_send(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setitem(
        feishu_cli.TOOL_HANDLERS["message"],
        "send_message",
        lambda args, _auth: {"success": True, "echo": args},
    )
    result = feishu_cli.execute(
        {
            "command": "domain",
            "domain": "communicate",
            "domain_action": "send",
            "args": {"content": "hello"},
        }
    )
    assert result["success"] is True
    assert result["echo"]["content"] == "hello"


def test_domain_schedule_query(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setitem(
        feishu_cli.TOOL_HANDLERS["calendar"],
        "query",
        lambda args, _auth: {"success": True, "echo": args},
    )
    result = feishu_cli.execute(
        {
            "command": "domain",
            "domain": "schedule",
            "domain_action": "query",
            "args": {"start": "2026-03-11"},
        }
    )
    assert result["success"] is True
    assert result["echo"]["start"] == "2026-03-11"


def test_domain_task_create(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setitem(
        feishu_cli.TOOL_HANDLERS["task"],
        "create",
        lambda args, _auth: {"success": True, "echo": args},
    )
    result = feishu_cli.execute(
        {
            "command": "domain",
            "domain": "task",
            "domain_action": "create",
            "args": {"summary": "Review PR"},
        }
    )
    assert result["success"] is True
    assert result["echo"]["summary"] == "Review PR"


def test_domain_document_write_markdown(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setitem(
        feishu_cli.TOOL_HANDLERS["doc"],
        "write_markdown",
        lambda args, _auth: {"success": True, "echo": args},
    )
    result = feishu_cli.execute(
        {
            "command": "domain",
            "domain": "document",
            "domain_action": "write-markdown",
            "args": {"document_id": "doccnxxxx", "content": "# Title"},
        }
    )
    assert result["success"] is True
    assert result["echo"]["document_id"] == "doccnxxxx"


def test_domain_knowledge_list_spaces(monkeypatch: pytest.MonkeyPatch):
    monkeypatch.setitem(
        feishu_cli.TOOL_HANDLERS["wiki"],
        "list_spaces",
        lambda args, _auth: {"success": True, "echo": args},
    )
    result = feishu_cli.execute(
        {
            "command": "domain",
            "domain": "knowledge",
            "domain_action": "list-spaces",
            "args": {},
        }
    )
    assert result["success"] is True


def test_domain_unknown_domain():
    result = feishu_cli.execute(
        {
            "command": "domain",
            "domain": "nonexistent",
            "domain_action": "foo",
            "args": {},
        }
    )
    assert result["success"] is False
    assert "unknown domain" in result["error"]
    assert "available_domains" in result


def test_domain_unknown_action():
    result = feishu_cli.execute(
        {
            "command": "domain",
            "domain": "communicate",
            "domain_action": "nonexistent",
            "args": {},
        }
    )
    assert result["success"] is False
    assert "unknown action" in result["error"]
    assert "available_actions" in result


def test_domain_cli_parsing():
    """Test that CLI argv parsing routes domain subcommands correctly."""
    request = feishu_cli._build_request_from_cli(["communicate", "send", '{"content": "hi"}'])
    assert request["command"] == "domain"
    assert request["domain"] == "communicate"
    assert request["domain_action"] == "send"
    assert request["args"]["content"] == "hi"


def test_domain_cli_parsing_schedule():
    request = feishu_cli._build_request_from_cli(["schedule", "query", '{"start": "today"}'])
    assert request["command"] == "domain"
    assert request["domain"] == "schedule"
    assert request["domain_action"] == "query"


def test_help_overview_includes_domains():
    result = feishu_cli.execute({"command": "help"})
    assert result["success"] is True
    assert "domains" in result
    domain_names = [d["domain"] for d in result["domains"]]
    for expected in ("communicate", "schedule", "task", "document", "knowledge"):
        assert expected in domain_names


def test_domain_commands_completeness():
    """Every DOMAIN_COMMANDS target must exist in TOOL_HANDLERS."""
    for domain, actions in feishu_cli.DOMAIN_COMMANDS.items():
        for action_name, (module, canonical) in actions.items():
            assert module in feishu_cli.TOOL_HANDLERS, f"{domain}.{action_name} -> missing module {module}"
            assert canonical in feishu_cli.TOOL_HANDLERS[module], (
                f"{domain}.{action_name} -> missing handler {module}.{canonical}"
            )
