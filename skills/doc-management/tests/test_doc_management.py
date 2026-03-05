"""Tests for doc-management skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("doc_management_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_read_doc_delegates_to_feishu_cli():
    with patch.object(_mod, "feishu_tool", return_value={"success": False, "error": "document_id is required"}) as mock:
        result = _mod.read_doc({})
        mock.assert_called_once_with("doc", "read", {})

    assert result["success"] is False


def test_create_doc_success():
    with patch.object(_mod, "feishu_tool", return_value={"success": True, "document": {"document_id": "doc_1"}}):
        result = _mod.create_doc({"title": "test"})

    assert result["success"] is True
    assert result["document"]["document_id"] == "doc_1"
