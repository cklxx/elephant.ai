"""Tests for audio-tts skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path
from unittest.mock import patch

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("audio_tts_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)


def test_speak_requires_text():
    result = _mod.speak({})
    assert result["success"] is False
    assert "text" in result["error"]


def test_speak_success_creates_output(tmp_path):
    output = tmp_path / "tts.m4a"

    def fake_run(cmd: list[str]):
        if cmd and cmd[0] == "afconvert":
            Path(cmd[-1]).write_bytes(b"audio")
        return 0, "", ""

    with patch.object(_mod, "_run", side_effect=fake_run):
        result = _mod.speak({"text": "hello", "output": str(output)})

    assert result["success"] is True
    assert result["audio_path"] == str(output)
    assert output.exists()
