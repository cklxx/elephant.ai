"""Tests for generate_agent_e2e_cases."""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path

_RUN_PATH = Path(__file__).resolve().parent.parent / "generate_agent_e2e_cases.py"
_spec = importlib.util.spec_from_file_location("generate_agent_e2e_cases", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
sys.modules[_spec.name] = _mod
_spec.loader.exec_module(_mod)


def test_discover_skills_reads_front_matter_and_entrypoint(tmp_path):
    skills_root = tmp_path / "skills"
    alpha = skills_root / "alpha-skill"
    alpha.mkdir(parents=True)
    (alpha / "SKILL.md").write_text(
        "---\nname: alpha\ndescription: Alpha desc\n---\n\n# alpha\n", encoding="utf-8"
    )
    (alpha / "run.py").write_text("print('ok')\n", encoding="utf-8")

    beta = skills_root / "beta-skill"
    beta.mkdir(parents=True)
    (beta / "SKILL.md").write_text("# beta\n", encoding="utf-8")

    discovered = _mod.discover_skills(tmp_path)
    assert [item.skill_name for item in discovered] == ["alpha", "beta-skill"]
    assert discovered[0].description == "Alpha desc"
    assert discovered[0].has_run_script is True
    assert discovered[1].has_run_script is False


def test_build_cases_generates_expected_exec_command():
    skills = [
        _mod.SkillMeta(
            skill_name="image-creation",
            skill_dir="image-creation",
            has_run_script=True,
            description="desc",
        ),
        _mod.SkillMeta(
            skill_name="video-production",
            skill_dir="video-production",
            has_run_script=False,
            description="desc",
        ),
    ]

    payload = _mod.build_cases(skills)
    cases = {item["skill_name"]: item for item in payload["cases"]}

    assert cases["image-creation"]["expected_exec_command"] == (
        "python3 skills/image-creation/run.py '{...}'"
    )
    assert cases["video-production"]["expected_exec_command"] == ""


def test_case_prompt_requires_skills_tool_only():
    meta = _mod.SkillMeta(
        skill_name="calendar-management",
        skill_dir="calendar-management",
        has_run_script=True,
        description="desc",
    )
    prompt = _mod._build_prompt(meta)
    assert "Call the `skills` tool with action=show and name=\"calendar-management\"" in prompt
    assert "Do not call any tool except `skills`." in prompt
    assert "python3 skills/calendar-management/run.py '{...}'" in prompt
