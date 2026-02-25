#!/usr/bin/env python3
"""Generate per-skill E2E agent cases for validating the `skills` tool."""

from __future__ import annotations

import argparse
import pathlib
import re
from dataclasses import dataclass
from typing import Any

import yaml


@dataclass
class SkillMeta:
    skill_name: str
    skill_dir: str
    has_run_script: bool
    description: str


def _repo_root() -> pathlib.Path:
    return pathlib.Path(__file__).resolve().parents[2]


def _read_front_matter(skill_md: pathlib.Path) -> dict[str, Any]:
    content = skill_md.read_text(encoding="utf-8")
    if not content.startswith("---\n"):
        return {}
    parts = content.split("\n---\n", 1)
    if len(parts) != 2:
        return {}
    header = parts[0].removeprefix("---\n")
    try:
        parsed = yaml.safe_load(header)
    except yaml.YAMLError:
        return {}
    if isinstance(parsed, dict):
        return parsed
    return {}


def discover_skills(root: pathlib.Path) -> list[SkillMeta]:
    skills_root = root / "skills"
    out: list[SkillMeta] = []
    for entry in sorted(skills_root.iterdir(), key=lambda p: p.name):
        if not entry.is_dir():
            continue
        skill_md = entry / "SKILL.md"
        if not skill_md.exists():
            continue
        meta = _read_front_matter(skill_md)
        skill_name = str(meta.get("name") or entry.name).strip()
        description = str(meta.get("description") or "").strip()
        out.append(
            SkillMeta(
                skill_name=skill_name,
                skill_dir=entry.name,
                has_run_script=(entry / "run.py").exists(),
                description=description,
            )
        )
    return out


def _case_id(skill_name: str) -> str:
    normalized = re.sub(r"[^a-z0-9]+", "-", skill_name.lower()).strip("-")
    return f"skills-tool-e2e-{normalized}"


def _build_prompt(skill: SkillMeta) -> str:
    if skill.has_run_script:
        exec_command = f"python3 skills/{skill.skill_dir}/run.py '{{...}}'"
        py_line = (
            "Set `has_python_entrypoint` to true and set `exec_command` to "
            f"\"{exec_command}\" exactly."
        )
    else:
        py_line = (
            "Set `has_python_entrypoint` to false and set `exec_command` to an empty string."
        )

    return (
        f"You are validating repository skill \"{skill.skill_name}\".\n"
        "Mandatory steps:\n"
        f"1) Call the `skills` tool with action=show and name=\"{skill.skill_name}\".\n"
        "2) Do not call any tool except `skills`.\n"
        "3) Return JSON only with keys:\n"
        "   skill, used_skills_tool, has_playbook, has_python_entrypoint, exec_command, summary\n"
        "4) Set `skill` exactly to the requested name and `used_skills_tool` to true.\n"
        "5) Set `has_playbook` to true if the skill content was loaded.\n"
        f"6) {py_line}\n"
        "7) Keep `summary` under 30 words."
    )


def build_cases(skills: list[SkillMeta]) -> dict[str, Any]:
    cases: list[dict[str, Any]] = []
    for skill in sorted(skills, key=lambda s: s.skill_name.lower()):
        cases.append(
            {
                "id": _case_id(skill.skill_name),
                "skill_name": skill.skill_name,
                "skill_dir": skill.skill_dir,
                "description": skill.description,
                "has_run_script": skill.has_run_script,
                "expected_exec_command": (
                    f"python3 skills/{skill.skill_dir}/run.py '{{...}}'"
                    if skill.has_run_script
                    else ""
                ),
                "prompt": _build_prompt(skill),
            }
        )
    return {
        "version": 1,
        "description": "Per-skill end-to-end agent cases validating `skills` tool effectiveness.",
        "cases": cases,
    }


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--output",
        default="evaluation/skills_e2e/cases.yaml",
        help="Output YAML file path, relative to repo root unless absolute.",
    )
    args = parser.parse_args()

    root = _repo_root()
    output = pathlib.Path(args.output)
    if not output.is_absolute():
        output = root / output
    output.parent.mkdir(parents=True, exist_ok=True)

    skills = discover_skills(root)
    payload = build_cases(skills)
    output.write_text(
        yaml.safe_dump(payload, sort_keys=False, allow_unicode=True), encoding="utf-8"
    )
    print(f"wrote {output} ({len(skills)} cases)")


if __name__ == "__main__":
    main()
