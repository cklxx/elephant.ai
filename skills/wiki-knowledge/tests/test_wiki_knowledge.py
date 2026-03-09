"""Tests for wiki-knowledge skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("wiki_knowledge_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_list_nodes_delegates_to_feishu_cli():
    with patch.object(_mod, "feishu_tool", return_value={"success": False, "error": "space_id is required"}) as mock:
        result = _mod.list_nodes({})
        mock.assert_called_once_with("wiki", "list_nodes", {})

    assert result["success"] is False


def test_list_spaces_success():
    with patch.object(_mod, "feishu_tool", return_value={"success": True, "spaces": [{"space_id": "w1"}], "count": 1}):
        result = _mod.list_spaces({})

    assert result["success"] is True
    assert result["count"] == 1
