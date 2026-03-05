"""Tests for doc-management skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("doc_management_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_read_doc_requires_document_id():
    result = _mod.read_doc({})
    assert result["success"] is False
    assert "document_id" in result["error"]


def test_create_doc_success():
    with patch.object(
        _mod,
        "_lark_api",
        return_value={"data": {"document": {"document_id": "doc_1"}}},
    ):
        result = _mod.create_doc({"title": "test"})

    assert result["success"] is True
    assert result["document"]["document_id"] == "doc_1"
