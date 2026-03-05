"""Tests for notebooklm-cli standalone runtime."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_SPEC = importlib.util.spec_from_file_location("notebooklm_cli_skill_run", _RUN_PATH)
_MOD = importlib.util.module_from_spec(_SPEC)
_SPEC.loader.exec_module(_MOD)


def _ok(command: str = "nlm --help") -> dict:
    return {
        "success": True,
        "command": command,
        "exit_code": 0,
        "stdout": "ok",
        "stderr": "",
        "hints": [],
    }


def test_non_object_args_rejected():
    result = _MOD.run([])
    assert result["success"] is False
    assert "object" in result["error"]


def test_cli_missing_returns_clear_error():
    with patch.object(_MOD, "_ensure_nlm_available", return_value={"success": False, "error": "missing"}):
        result = _MOD.run({"action": "notebook", "op": "list"})
    assert result["success"] is False
    assert result["error"] == "missing"


def test_help_works_without_nlm_binary():
    with patch.object(_MOD, "_ensure_nlm_available", return_value={"success": False, "error": "missing"}):
        result = _MOD.run({"action": "help", "topic": "schema"})
    assert result["success"] is True
    assert "schema" in result


def test_help_progressive_dispatch():
    result = _MOD.run({"action": "help", "topic": "progressive"})
    assert result["success"] is True
    assert len(result["steps"]) == 8
    assert result["steps"][0]["topic"] == "overview"
    assert result["steps"][1]["topic"] == "auth"


def test_notebook_create_requires_title():
    with patch.object(_MOD, "_ensure_nlm_available", return_value=None):
        result = _MOD.run({"action": "notebook", "op": "create"})
    assert result["success"] is False
    assert "title is required" in result["error"]


def test_notebook_delete_requires_confirm():
    with patch.object(_MOD, "_ensure_nlm_available", return_value=None):
        result = _MOD.run({"action": "notebook", "op": "delete", "notebook_id": "nb-1"})
    assert result["success"] is False
    assert "confirm=true" in result["error"]


def test_source_add_url_dispatch():
    with patch.object(_MOD, "_ensure_nlm_available", return_value=None), patch.object(_MOD, "_run_nlm", return_value=_ok()) as mock:
        result = _MOD.run(
            {
                "action": "source",
                "op": "add_url",
                "notebook_id": "nb-1",
                "url": "https://example.com",
            }
        )
    assert result["success"] is True
    mock.assert_called_once_with(["source", "add", "nb-1", "--url", "https://example.com"])


def test_source_add_urls_batch_dispatch():
    with patch.object(_MOD, "_ensure_nlm_available", return_value=None), patch.object(_MOD, "_run_nlm", return_value=_ok()) as mock:
        result = _MOD.run(
            {
                "action": "source",
                "op": "add_url",
                "notebook_id": "nb-1",
                "urls": ["https://a.com", "https://b.com"],
                "wait": True,
            }
        )
    assert result["success"] is True
    mock.assert_called_once_with(["source", "add", "nb-1", "--url", "https://a.com", "--url", "https://b.com", "--wait"])


def test_source_list_with_flags_and_profile():
    with patch.object(_MOD, "_ensure_nlm_available", return_value=None), patch.object(_MOD, "_run_nlm", return_value=_ok()) as mock:
        result = _MOD.run(
            {
                "action": "source",
                "op": "list",
                "notebook_id": "nb-1",
                "json": True,
                "quiet": True,
                "profile": "p1",
            }
        )
    assert result["success"] is True
    mock.assert_called_once_with(["source", "list", "nb-1", "--json", "--quiet", "--profile", "p1"])


def test_query_shorthand_dispatch():
    with patch.object(_MOD, "_ensure_nlm_available", return_value=None), patch.object(_MOD, "_run_nlm", return_value=_ok()) as mock:
        result = _MOD.run({"action": "query", "notebook_id": "nb-1", "question": "what changed?"})
    assert result["success"] is True
    mock.assert_called_once_with(["notebook", "query", "nb-1", "what changed?"])


def test_profile_delete_requires_and_appends_confirm():
    with patch.object(_MOD, "_ensure_nlm_available", return_value=None), patch.object(_MOD, "_run_nlm", return_value=_ok()) as mock:
        result = _MOD.run({"action": "auth", "op": "profile_delete", "profile": "p1", "confirm": True})
    assert result["success"] is True
    mock.assert_called_once_with(["login", "profile", "delete", "p1", "--confirm"])


def test_report_create_supports_optional_fields():
    with patch.object(_MOD, "_ensure_nlm_available", return_value=None), patch.object(_MOD, "_run_nlm", return_value=_ok()) as mock:
        result = _MOD.run(
            {
                "action": "report",
                "op": "create",
                "notebook_id": "nb-1",
                "confirm": True,
                "format": "Briefing Doc",
                "language": "zh-CN",
                "source_ids": ["s1", "s2"],
            }
        )
    assert result["success"] is True
    mock.assert_called_once_with(
        ["report", "create", "nb-1", "--confirm", "--format", "Briefing Doc", "--language", "zh-CN", "--source-ids", "s1,s2"]
    )


def test_studio_delete_requires_confirm():
    with patch.object(_MOD, "_ensure_nlm_available", return_value=None):
        result = _MOD.run({"action": "studio", "op": "delete", "notebook_id": "nb-1", "artifact_id": "a-1"})
    assert result["success"] is False
    assert "confirm=true" in result["error"]


def test_raw_disallow_interactive_chat_start():
    with patch.object(_MOD, "_ensure_nlm_available", return_value=None):
        result = _MOD.run({"action": "raw", "argv": "nlm chat start"})
    assert result["success"] is False
    assert "not supported" in result["error"]


def test_raw_delete_requires_confirm():
    with patch.object(_MOD, "_ensure_nlm_available", return_value=None):
        result = _MOD.run({"action": "raw", "argv": ["nlm", "source", "delete", "s-1"]})
    assert result["success"] is False
    assert "confirm=true" in result["error"]


def test_raw_delete_auto_add_confirm():
    with patch.object(_MOD, "_ensure_nlm_available", return_value=None), patch.object(_MOD, "_run_nlm", return_value=_ok()) as mock:
        result = _MOD.run({"action": "raw", "argv": "nlm source delete s-1", "confirm": True})
    assert result["success"] is True
    mock.assert_called_once_with(["source", "delete", "s-1", "--confirm"])


def test_raw_delete_with_existing_flag_does_not_require_confirm_field():
    with patch.object(_MOD, "_ensure_nlm_available", return_value=None), patch.object(_MOD, "_run_nlm", return_value=_ok()) as mock:
        result = _MOD.run({"action": "raw", "argv": ["nlm", "source", "delete", "s-1", "--confirm"]})
    assert result["success"] is True
    mock.assert_called_once_with(["source", "delete", "s-1", "--confirm"])


def test_help_schema_returns_structured_commands():
    result = _MOD.run({"command": "help", "topic": "schema"})
    assert result["success"] is True
    assert "notebook" in result["schema"]
    assert "source" in result["schema"]
