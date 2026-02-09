"""Tests for diagram-to-image skill."""

from __future__ import annotations

import importlib.util
import os
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("diagram_to_image_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

render_mermaid = _mod.render_mermaid
run = _mod.run


class TestRenderMermaid:
    def test_missing_code(self):
        result = render_mermaid({})
        assert result["success"] is False
        assert "code" in result["error"]

    def test_mmdc_not_found(self):
        with patch("subprocess.run", side_effect=FileNotFoundError):
            result = render_mermaid({"code": "graph TD; A-->B"})
            assert result["success"] is False
            assert "mmdc not found" in result["error"]

    def test_mmdc_timeout(self):
        import subprocess
        with patch("subprocess.run", side_effect=subprocess.TimeoutExpired("mmdc", 30)):
            result = render_mermaid({"code": "graph TD; A-->B"})
            assert result["success"] is False
            assert "timeout" in result["error"]

    def test_mmdc_failure(self):
        mock_result = MagicMock()
        mock_result.returncode = 1
        mock_result.stderr = "parse error"
        with patch("subprocess.run", return_value=mock_result):
            result = render_mermaid({"code": "invalid"})
            assert result["success"] is False
            assert "parse error" in result["error"]

    def test_success(self, tmp_path):
        output_path = str(tmp_path / "test.png")
        # Create the output file to simulate mmdc creating it
        mock_result = MagicMock()
        mock_result.returncode = 0

        def fake_run(*args, **kwargs):
            Path(output_path).write_bytes(b"PNG")
            return mock_result

        with patch("subprocess.run", side_effect=fake_run):
            result = render_mermaid({"code": "graph TD; A-->B", "output": output_path})
            assert result["success"] is True
            assert result["path"] == output_path
            assert result["format"] == "png"

    def test_svg_format(self, tmp_path):
        output_path = str(tmp_path / "test.svg")
        mock_result = MagicMock()
        mock_result.returncode = 0

        def fake_run(*args, **kwargs):
            Path(output_path).write_text("<svg></svg>")
            return mock_result

        with patch("subprocess.run", side_effect=fake_run):
            result = render_mermaid({"code": "graph TD; A-->B", "output": output_path, "format": "svg"})
            assert result["success"] is True
            assert result["format"] == "svg"


class TestRun:
    def test_default_action_is_render(self):
        result = run({"code": ""})
        assert result["success"] is False
        assert "code" in result["error"]

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
