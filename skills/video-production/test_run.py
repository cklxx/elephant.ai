#!/usr/bin/env python3
"""Unit tests for video-production skill runtime behavior."""

from __future__ import annotations

import importlib.util
import json
import os
import tempfile
from pathlib import Path
import unittest
import urllib.request


def _load_module():
    module_path = Path(__file__).with_name("run.py")
    spec = importlib.util.spec_from_file_location("video_production_run", module_path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"failed to load module from {module_path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class VideoProductionRunTests(unittest.TestCase):
    def setUp(self) -> None:
        self.mod = _load_module()
        self.orig_urlopen = self.mod.urllib.request.urlopen
        os.environ["ARK_API_KEY"] = "test-key"
        os.environ["SEEDANCE_ENDPOINT_ID"] = "seedance-test-model"

    def tearDown(self) -> None:
        self.mod.urllib.request.urlopen = self.orig_urlopen

    def test_generate_persists_downloaded_video(self) -> None:
        class _Resp:
            def __init__(self, payload: bytes):
                self._payload = payload

            def __enter__(self):
                return self

            def __exit__(self, _exc_type, _exc, _tb):
                return False

            def read(self):
                return self._payload

        def _mock_urlopen(req, timeout=0):
            if isinstance(req, urllib.request.Request):
                payload = json.dumps({"data": [{"url": "https://example.com/video.mp4"}]}).encode()
                return _Resp(payload)
            return _Resp(b"fake-mp4-bytes")

        self.mod.urllib.request.urlopen = _mock_urlopen

        with tempfile.TemporaryDirectory() as tmp_dir:
            output = str(Path(tmp_dir) / "video.mp4")
            result = self.mod.generate({"prompt": "cat", "output": output})
            self.assertTrue(result.get("success"), result)
            self.assertTrue(Path(output).exists())
            self.assertGreater(Path(output).stat().st_size, 0)

    def test_generate_fails_when_video_url_missing(self) -> None:
        class _Resp:
            def __enter__(self):
                return self

            def __exit__(self, _exc_type, _exc, _tb):
                return False

            def read(self):
                return json.dumps({"data": [{}]}).encode()

        self.mod.urllib.request.urlopen = lambda *_args, **_kwargs: _Resp()

        result = self.mod.generate({"prompt": "cat"})
        self.assertFalse(result.get("success"), result)
        self.assertIn("missing video url", result.get("error", ""))


if __name__ == "__main__":
    unittest.main()
