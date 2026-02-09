"""Tests for a2ui-templates skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("a2ui_templates_run", _RUN_PATH)
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

    def test_flowchart(self):
        result = generate({"template": "flowchart"})
        assert result["success"] is True
        assert result["component"]["component"] == "Flowchart"

    def test_form(self):
        result = generate({"template": "form"})
        assert result["success"] is True
        assert result["component"]["component"] == "Form"

    def test_dashboard(self):
        result = generate({"template": "dashboard"})
        assert result["success"] is True
        assert "widgets" in result["component"]["props"]

    def test_cards(self):
        result = generate({"template": "cards"})
        assert result["success"] is True
        assert result["component"]["component"] == "CardList"

    def test_gallery(self):
        result = generate({"template": "gallery"})
        assert result["success"] is True
        assert result["component"]["component"] == "Gallery"

    def test_custom_data_replaces_items(self):
        data = [{"title": "Custom"}]
        result = generate({"template": "cards", "data": data})
        assert result["component"]["props"]["items"] == data

    def test_custom_data_replaces_images(self):
        data = [{"url": "http://img.png"}]
        result = generate({"template": "gallery", "data": data})
        assert result["component"]["props"]["images"] == data

    def test_deep_copy_isolation(self):
        result1 = generate({"template": "cards", "data": [{"x": 1}]})
        result2 = generate({"template": "cards"})
        assert result1["component"]["props"]["items"] != result2["component"]["props"]["items"]


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
