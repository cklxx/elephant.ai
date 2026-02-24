"""Tests for contact-lookup skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("contact_lookup_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_get_user_requires_user_id():
    result = _mod.get_user({})
    assert result["success"] is False
    assert "user_id" in result["error"]


def test_get_user_success():
    with patch.object(
        _mod,
        "_lark_api",
        return_value={"data": {"user": {"user_id": "u_1", "name": "alice"}}},
    ):
        result = _mod.get_user({"user_id": "u_1"})

    assert result["success"] is True
    assert result["user"]["user_id"] == "u_1"
