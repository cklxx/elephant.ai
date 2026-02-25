"""Tests for json-render-templates skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("json_render_templates_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

generate = _mod.generate
run = _mod.run


class TestGenerate:
    def test_no_template_lists_available(self):
        result = generate({})
        assert result["success"] is True
        assert "available" in result
        assert "flowchart" in result["available"]

    def test_unknown_template(self):
        result = generate({"template": "nonexistent"})
        assert result["success"] is False
        assert "unknown template" in result["error"]

    def test_flowchart(self):
        result = generate({"template": "flowchart"})
        assert result["success"] is True
        assert result["template"]["type"] == "flowchart"
        assert len(result["template"]["nodes"]) == 3

    def test_form(self):
        result = generate({"template": "form"})
        assert result["success"] is True
        assert result["template"]["type"] == "form"
        assert len(result["template"]["fields"]) == 3

    def test_dashboard(self):
        result = generate({"template": "dashboard"})
        assert result["success"] is True
        assert "widgets" in result["template"]

    def test_cards(self):
        result = generate({"template": "cards"})
        assert result["success"] is True
        assert "items" in result["template"]

    def test_table(self):
        result = generate({"template": "table"})
        assert result["success"] is True
        assert "columns" in result["template"]

    def test_custom_title(self):
        result = generate({"template": "form", "title": "My Form"})
        assert result["template"]["title"] == "My Form"

    def test_custom_data_rows(self):
        data = [{"name": "A", "value": 1}]
        result = generate({"template": "table", "data": data})
        assert result["template"]["rows"] == data

    def test_custom_data_items(self):
        data = [{"title": "Custom"}]
        result = generate({"template": "cards", "data": data})
        assert result["template"]["items"] == data

    def test_deep_copy_isolation(self):
        result1 = generate({"template": "table", "data": [{"x": 1}]})
        result2 = generate({"template": "table"})
        assert result1["template"]["rows"] != result2["template"]["rows"]


class TestRun:
    def test_default_action_is_generate(self):
        result = run({})
        assert result["success"] is True
        assert "available" in result

    def test_list_action(self):
        result = run({"action": "list"})
        assert result["success"] is True
        assert "templates" in result

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
