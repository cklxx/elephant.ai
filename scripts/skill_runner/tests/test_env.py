"""Tests for skill_runner.env."""

from __future__ import annotations

import importlib.util
import os
import sys
from pathlib import Path

_ENV_PATH = Path(__file__).resolve().parent.parent / "env.py"
_spec = importlib.util.spec_from_file_location("skill_runner_env", _ENV_PATH)
_mod = importlib.util.module_from_spec(_spec)
sys.modules[_spec.name] = _mod
_spec.loader.exec_module(_mod)


def _fake_load_dotenv(*, dotenv_path, override=False):
    for raw_line in Path(dotenv_path).read_text(encoding="utf-8").splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        key = key.strip()
        value = value.strip()
        if override or key not in os.environ:
            os.environ[key] = value
    return True


def test_find_dotenv_searches_parent_directories(tmp_path):
    root = tmp_path / "repo"
    nested = root / "skills" / "image-creation"
    nested.mkdir(parents=True)
    env_file = root / ".env"
    env_file.write_text("A=1\n", encoding="utf-8")

    found = _mod.find_dotenv(nested / "run.py")
    assert found == env_file


def test_load_repo_dotenv_preserves_existing_values_by_default(tmp_path, monkeypatch):
    env_file = tmp_path / ".env"
    env_file.write_text("ARK_API_KEY=from-file\n", encoding="utf-8")
    monkeypatch.setenv("ARK_API_KEY", "from-env")
    _mod._LOADED_PATHS.clear()
    monkeypatch.setattr(_mod, "_resolve_load_dotenv", lambda: _fake_load_dotenv)

    loaded = _mod.load_repo_dotenv(env_file, override=False)
    assert loaded == env_file
    assert os.environ["ARK_API_KEY"] == "from-env"


def test_load_repo_dotenv_override_true_replaces_existing_value(tmp_path, monkeypatch):
    env_file = tmp_path / ".env"
    env_file.write_text("ARK_API_KEY=from-file\n", encoding="utf-8")
    monkeypatch.setenv("ARK_API_KEY", "from-env")
    _mod._LOADED_PATHS.clear()
    monkeypatch.setattr(_mod, "_resolve_load_dotenv", lambda: _fake_load_dotenv)

    loaded = _mod.load_repo_dotenv(env_file, override=True)
    assert loaded == env_file
    assert os.environ["ARK_API_KEY"] == "from-file"


def test_load_repo_dotenv_is_idempotent_without_override(tmp_path, monkeypatch):
    env_file = tmp_path / ".env"
    env_file.write_text("A=1\n", encoding="utf-8")
    _mod._LOADED_PATHS.clear()
    call_count = {"n": 0}

    def fake_loader(*, dotenv_path, override=False):
        call_count["n"] += 1
        return _fake_load_dotenv(dotenv_path=dotenv_path, override=override)

    monkeypatch.setattr(_mod, "_resolve_load_dotenv", lambda: fake_loader)

    _mod.load_repo_dotenv(env_file, override=False)
    _mod.load_repo_dotenv(env_file, override=False)
    assert call_count["n"] == 1


def test_load_repo_dotenv_returns_none_when_dotenv_missing(tmp_path, monkeypatch):
    _mod._LOADED_PATHS.clear()
    monkeypatch.setattr(_mod, "_resolve_load_dotenv", lambda: _fake_load_dotenv)
    loaded = _mod.load_repo_dotenv(tmp_path / "missing.py")
    assert loaded is None
