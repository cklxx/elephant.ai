"""Tests for ppt-deck skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("ppt_deck_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

outline = _mod.outline
list_templates = _mod.list_templates
run = _mod.run


class TestOutline:
    def test_missing_topic(self):
        result = outline({})
        assert result["success"] is False
        assert "topic" in result["error"]

    def test_basic_outline(self):
        result = outline({"topic": "Q1 Strategy"})
        assert result["success"] is True
        assert result["topic"] == "Q1 Strategy"
        assert result["template"] == "scqa"
        assert len(result["pages"]) >= 3  # cover + sections + cta
        assert result["pages"][0]["type"] == "cover"
        assert result["pages"][-1]["type"] == "cta"
        assert "outline_prompt" in result

    def test_pyramid_template(self):
        result = outline({"topic": "Test", "template": "pyramid"})
        assert result["template"] == "pyramid"
        # Pyramid has 5 sections + cover + tl_dr + cta = 8
        assert len(result["pages"]) == 8

    def test_before_after_bridge(self):
        result = outline({"topic": "Test", "template": "before_after_bridge"})
        # 3 sections + cover + tl_dr + cta = 6
        assert len(result["pages"]) == 6

    def test_brand_info(self):
        result = outline({
            "topic": "Test",
            "font": "Helvetica",
            "colors": ["#000", "#fff"],
            "logo": "logo.png",
        })
        assert result["brand"]["font"] == "Helvetica"
        assert result["brand"]["colors"] == ["#000", "#fff"]

    def test_constraints(self):
        result = outline({"topic": "Test", "rule_10_20_30": True, "max_pages": 10})
        assert result["constraints"]["rule_10_20_30"] is True
        assert result["constraints"]["max_pages"] == 10

    def test_unknown_template_falls_back(self):
        result = outline({"topic": "Test", "template": "nonexistent"})
        assert result["success"] is True
        # Falls back to SCQA (4 sections + cover + tl_dr + cta = 7)
        assert len(result["pages"]) == 7


class TestListTemplates:
    def test_returns_templates(self):
        result = list_templates({})
        assert result["success"] is True
        assert "scqa" in result["story_templates"]
        assert "pyramid" in result["story_templates"]
        assert "cover" in result["page_types"]


class TestRun:
    def test_default_action_is_outline(self):
        result = run({})
        assert result["success"] is False  # missing topic

    def test_list_action(self):
        result = run({"action": "list"})
        assert result["success"] is True

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
