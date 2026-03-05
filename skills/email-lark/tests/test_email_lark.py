"""Tests for email-lark skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("email_lark_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_get_mailgroup_delegates_to_feishu_cli():
    with patch.object(_mod, "feishu_tool", return_value={"success": False, "error": "mailgroup_id is required"}) as mock:
        result = _mod.get_mailgroup({})
        mock.assert_called_once_with("mail", "get_mailgroup", {})

    assert result["success"] is False


def test_list_mailgroups_success():
    with patch.object(_mod, "feishu_tool", return_value={"success": True, "mailgroups": [{"id": "g1"}], "has_more": False}):
        result = _mod.list_mailgroups({})

    assert result["success"] is True
    assert len(result["mailgroups"]) == 1
