"""Tests for wiki-knowledge skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("wiki_knowledge_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_list_nodes_requires_space_id():
    result = _mod.list_nodes({})
    assert result["success"] is False
    assert "space_id" in result["error"]


def test_list_spaces_success():
    with patch.object(
        _mod,
        "_lark_api",
        return_value={"data": {"items": [{"space_id": "w1"}]}},
    ):
        result = _mod.list_spaces({})

    assert result["success"] is True
    assert result["count"] == 1
