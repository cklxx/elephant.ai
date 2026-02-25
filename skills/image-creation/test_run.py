#!/usr/bin/env python3
"""Unit tests for image-creation skill runtime behavior."""

from __future__ import annotations

import base64
import importlib.util
import tempfile
from pathlib import Path
import unittest


def _load_module():
    module_path = Path(__file__).with_name("run.py")
    spec = importlib.util.spec_from_file_location("image_creation_run", module_path)
    if spec is None or spec.loader is None:
        raise RuntimeError(f"failed to load module from {module_path}")
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


class ImageCreationRunTests(unittest.TestCase):
    def setUp(self) -> None:
        self.mod = _load_module()
        self.orig_ark_request = self.mod._ark_request
        self.orig_urlopen = self.mod.urllib.request.urlopen

    def tearDown(self) -> None:
        self.mod._ark_request = self.orig_ark_request
        self.mod.urllib.request.urlopen = self.orig_urlopen

    def test_generate_persists_b64_image(self) -> None:
        payload = base64.b64encode(b"fake-png-bytes").decode()
        self.mod._ark_request = lambda _endpoint, _body: {"data": [{"b64_json": payload}]}

        with tempfile.TemporaryDirectory() as tmp_dir:
            output = str(Path(tmp_dir) / "cat.png")
            result = self.mod.generate({"prompt": "cat", "output": output})
            self.assertTrue(result.get("success"), result)
            self.assertTrue(Path(output).exists())
            self.assertGreater(Path(output).stat().st_size, 0)

    def test_generate_downloads_url_when_b64_absent(self) -> None:
        self.mod._ark_request = lambda _endpoint, _body: {"data": [{"url": "https://example.com/cat.png"}]}

        class _Resp:
            def __enter__(self):
                return self

            def __exit__(self, _exc_type, _exc, _tb):
                return False

            def read(self):
                return b"url-image-bytes"

        self.mod.urllib.request.urlopen = lambda *_args, **_kwargs: _Resp()

        with tempfile.TemporaryDirectory() as tmp_dir:
            output = str(Path(tmp_dir) / "cat-url.png")
            result = self.mod.generate({"prompt": "cat", "output": output})
            self.assertTrue(result.get("success"), result)
            self.assertTrue(Path(output).exists())
            self.assertEqual(Path(output).read_bytes(), b"url-image-bytes")

    def test_generate_fails_without_image_payload(self) -> None:
        self.mod._ark_request = lambda _endpoint, _body: {"data": [{"foo": "bar"}]}

        with tempfile.TemporaryDirectory() as tmp_dir:
            output = str(Path(tmp_dir) / "missing.png")
            result = self.mod.generate({"prompt": "cat", "output": output})
            self.assertFalse(result.get("success"), result)
            self.assertIn("missing both b64_json and url", result.get("error", ""))
            self.assertFalse(Path(output).exists())


if __name__ == "__main__":
    unittest.main()
