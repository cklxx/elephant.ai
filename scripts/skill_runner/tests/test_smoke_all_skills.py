"""Tests for skill_runner.smoke_all_skills."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path

_SMOKE_PATH = Path(__file__).resolve().parent.parent / "smoke_all_skills.py"
_spec = importlib.util.spec_from_file_location("skill_runner_smoke", _SMOKE_PATH)
_mod = importlib.util.module_from_spec(_spec)
sys.modules[_spec.name] = _mod
_spec.loader.exec_module(_mod)


def _write_script(path: Path, source: str) -> None:
    path.write_text(source, encoding="utf-8")
    path.chmod(0o755)


def test_run_one_accepts_valid_skill_contract(tmp_path):
    run_py = tmp_path / "run.py"
    _write_script(
        run_py,
        """
import json
print(json.dumps({"success": True, "data": {"ok": 1}}))
""".strip()
        + "\n",
    )

    result = _mod._run_one(sys.executable, run_py, "{}", timeout=5)
    assert result.contract_ok is True
    assert result.success_value is True
    assert result.returncode == 0


def test_run_one_rejects_non_json_output(tmp_path):
    run_py = tmp_path / "run.py"
    _write_script(run_py, "print('not json')\n")

    result = _mod._run_one(sys.executable, run_py, "{}", timeout=5)
    assert result.contract_ok is False
    assert "invalid JSON output" in result.error


def test_discover_run_scripts_lists_skill_entries(tmp_path):
    (tmp_path / "a").mkdir(parents=True)
    (tmp_path / "a" / "run.py").write_text("# a\n", encoding="utf-8")
    (tmp_path / "b").mkdir(parents=True)
    (tmp_path / "b" / "README.md").write_text("# b\n", encoding="utf-8")

    found = _mod._discover_run_scripts(tmp_path)
    assert [p.parent.name for p in found] == ["a"]
