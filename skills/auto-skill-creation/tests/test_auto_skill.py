"""Tests for auto-skill-creation skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path

import pytest

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("auto_skill_creation_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

create = _mod.create
run = _mod.run


@pytest.fixture(autouse=True)
def _skills_dir(tmp_path, monkeypatch):
    monkeypatch.setattr(_mod, "_SKILLS_DIR", tmp_path)


class TestCreate:
    def test_missing_name(self):
        result = create({"description": "test"})
        assert result["success"] is False
        assert "name" in result["error"]

    def test_missing_description(self):
        result = create({"name": "test-skill"})
        assert result["success"] is False
        assert "description" in result["error"]

    def test_creates_scaffold(self, tmp_path):
        result = create({"name": "my-skill", "description": "A test skill"})
        assert result["success"] is True
        skill_dir = tmp_path / "my-skill"
        assert skill_dir.exists()
        assert (skill_dir / "SKILL.md").exists()
        assert (skill_dir / "run.py").exists()
        assert (skill_dir / "tests" / "__init__.py").exists()
        assert (skill_dir / "tests" / "test_my_skill.py").exists()

    def test_skill_md_content(self, tmp_path):
        create({"name": "cool-tool", "description": "Does cool stuff", "keywords": ["cool", "tool"]})
        content = (tmp_path / "cool-tool" / "SKILL.md").read_text()
        assert "cool-tool" in content
        assert "Does cool stuff" in content
        assert "cool" in content

    def test_run_py_content(self, tmp_path):
        create({"name": "cool-tool", "description": "Does cool stuff"})
        content = (tmp_path / "cool-tool" / "run.py").read_text()
        assert "cool-tool" in content
        assert "def run" in content
        assert "def main" in content

    def test_already_exists(self, tmp_path):
        (tmp_path / "existing").mkdir()
        result = create({"name": "existing", "description": "test"})
        assert result["success"] is False
        assert "already exists" in result["error"]

    def test_files_list_in_result(self, tmp_path):
        result = create({"name": "new-skill", "description": "test"})
        assert "SKILL.md" in result["files"]
        assert "run.py" in result["files"]


class TestRun:
    def test_default_action_is_create(self):
        result = run({})
        assert result["success"] is False  # missing name/description

    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
