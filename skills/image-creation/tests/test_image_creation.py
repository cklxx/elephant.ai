"""Tests for image-creation skill."""

from __future__ import annotations

import base64
import importlib.util
import json
import sys
from pathlib import Path
from unittest.mock import MagicMock, patch

import pytest

# Load run.py by absolute path to avoid collisions with other skills' run.py
_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("image_creation_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

generate = _mod.generate
refine = _mod.refine
run = _mod.run


class TestGenerate:
    def test_missing_prompt(self):
        result = generate({})
        assert result["success"] is False
        assert "prompt" in result["error"]

    def test_missing_endpoint(self):
        with patch.dict("os.environ", {}, clear=True):
            result = generate({"prompt": "a cat"})
            assert result["success"] is False
            assert "SEEDREAM_TEXT_ENDPOINT_ID" in result["error"]

    def test_successful_generation(self, tmp_path):
        fake_img = base64.b64encode(b"\x89PNG\r\n\x1a\n" + b"\x00" * 100).decode()
        mock_resp = MagicMock()
        mock_resp.read.return_value = json.dumps({
            "data": [{"b64_json": fake_img}]
        }).encode()
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = MagicMock(return_value=False)

        output = str(tmp_path / "test.png")
        with patch.dict("os.environ", {
            "ARK_API_KEY": "test-key",
            "SEEDREAM_TEXT_ENDPOINT_ID": "ep-test",
        }):
            with patch("urllib.request.urlopen", return_value=mock_resp):
                result = generate({"prompt": "a cat", "output": output})
                assert result["success"] is True
                assert result["image_path"] == output
                assert Path(output).exists()

    def test_api_error(self):
        import urllib.error

        with patch.dict("os.environ", {
            "ARK_API_KEY": "test-key",
            "SEEDREAM_TEXT_ENDPOINT_ID": "ep-test",
        }):
            with patch("urllib.request.urlopen", side_effect=urllib.error.URLError("timeout")):
                result = generate({"prompt": "a cat"})
                assert result["success"] is False

    def test_no_image_returned(self):
        mock_resp = MagicMock()
        mock_resp.read.return_value = json.dumps({"data": []}).encode()
        mock_resp.__enter__ = lambda s: s
        mock_resp.__exit__ = MagicMock(return_value=False)

        with patch.dict("os.environ", {
            "ARK_API_KEY": "test-key",
            "SEEDREAM_TEXT_ENDPOINT_ID": "ep-test",
        }):
            with patch("urllib.request.urlopen", return_value=mock_resp):
                result = generate({"prompt": "a cat"})
                assert result["success"] is False
                assert "no image" in result["error"]


class TestRefine:
    def test_missing_params(self):
        assert refine({})["success"] is False
        assert refine({"image_path": "/tmp/x.png"})["success"] is False
        assert refine({"prompt": "test"})["success"] is False

    def test_missing_endpoint(self, tmp_path):
        img = tmp_path / "src.png"
        img.write_bytes(b"\x89PNG")
        with patch.dict("os.environ", {}, clear=True):
            result = refine({"image_path": str(img), "prompt": "add stars"})
            assert result["success"] is False


class TestRunDispatch:
    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False

    def test_default_action_is_generate(self):
        with patch.object(_mod, "generate", return_value={"success": True}) as mock:
            run({})
            mock.assert_called_once()
