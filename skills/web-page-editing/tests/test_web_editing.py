"""Tests for web-page-editing skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("web_page_editing_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

extract = _mod.extract
generate = _mod.generate
run = _mod.run


class TestExtract:
    def test_missing_html(self):
        result = extract({})
        assert result["success"] is False

    def test_extracts_text(self):
        result = extract({"html": "<p>Hello <b>World</b></p>"})
        assert result["success"] is True
        assert "Hello" in result["text"]
        assert "World" in result["text"]
        assert "<" not in result["text"]

    def test_extracts_title(self):
        result = extract({"html": "<html><title>My Page</title><body>x</body></html>"})
        assert result["title"] == "My Page"

    def test_extracts_headings(self):
        result = extract({"html": "<h1>Title</h1><h2>Subtitle</h2>"})
        assert result["headings"] == ["Title", "Subtitle"]

    def test_extracts_links(self):
        result = extract({"html": '<a href="https://example.com">Click</a>'})
        assert len(result["links"]) == 1
        assert result["links"][0]["url"] == "https://example.com"
        assert result["links"][0]["text"] == "Click"

    def test_handles_entities(self):
        result = extract({"html": "<p>A &amp; B</p>"})
        assert "A & B" in result["text"]


class TestGenerate:
    def test_unknown_template(self):
        result = generate({"template": "nonexistent"})
        assert result["success"] is False

    def test_landing_template(self):
        result = generate({"template": "landing", "title": "My Site"})
        assert result["success"] is True
        assert "My Site" in result["html"]
        assert "<html" in result["html"]

    def test_report_template(self):
        result = generate({"template": "report", "title": "Q1 Report"})
        assert result["success"] is True
        assert "Q1 Report" in result["html"]

    def test_default_template(self):
        result = generate({})
        assert result["success"] is True
        assert result["template"] == "landing"


class TestRun:
    def test_default_action_is_extract(self):
        result = run({})
        assert result["success"] is False  # missing html

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
