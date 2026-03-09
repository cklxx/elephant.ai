"""Tests for contact-lookup skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("contact_lookup_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_get_user_delegates_to_feishu_cli():
    with patch.object(_mod, "feishu_tool", return_value={"success": True, "user": {"user_id": "u_1", "name": "alice"}}) as mock:
        result = _mod.get_user({"user_id": "u_1"})
        mock.assert_called_once_with("contact", "get_user", {"user_id": "u_1"})

    assert result["success"] is True
    assert result["user"]["user_id"] == "u_1"


def test_run_unknown_action():
    result = _mod.run({"action": "invalid"})
    assert result["success"] is False
