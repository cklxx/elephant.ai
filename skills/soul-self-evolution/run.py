#!/usr/bin/env python3
"""soul-self-evolution skill — 受控 SOUL 自演进。"""

from __future__ import annotations

from pathlib import Path
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv

load_repo_dotenv(__file__)

import hashlib
import json
import os
import time
from pathlib import Path
from typing import Any

_DEFAULT_IMMUTABLE_SECTIONS = ["## Safety Guardrails", "## Non-Negotiable Rules"]


def _resolve_soul_path(args: dict[str, Any]) -> Path:
    explicit = str(args.get("path", "")).strip()
    if explicit:
        return Path(explicit)

    env_path = str(os.environ.get("ALEX_SOUL_PATH", "")).strip()
    if env_path:
        return Path(env_path)

    repo_candidate = Path("docs/reference/SOUL.md")
    if repo_candidate.exists():
        return repo_candidate
    return Path("SOUL.md")


def _validate_soul_target(path: Path) -> bool:
    return path.name == "SOUL.md"


def _checkpoint_dir() -> Path:
    root = os.environ.get("ALEX_SOUL_CHECKPOINT_DIR", "~/.alex/soul-checkpoints")
    directory = Path(os.path.expanduser(root))
    directory.mkdir(parents=True, exist_ok=True)
    return directory


def _create_checkpoint(path: Path, content: str) -> Path:
    digest = hashlib.sha1(content.encode("utf-8")).hexdigest()[:8]
    stamp = time.strftime("%Y%m%d-%H%M%S")
    checkpoint = _checkpoint_dir() / f"{stamp}-{digest}.md"
    checkpoint.write_text(content, encoding="utf-8")
    return checkpoint


def _replace_section(content: str, section: str, new_body: str) -> str:
    lines = content.splitlines()
    target = section.strip()
    if not target.startswith("## "):
        target = f"## {target.lstrip('# ').strip()}"

    start_idx = -1
    for idx, line in enumerate(lines):
        if line.strip() == target:
            start_idx = idx
            break

    new_block = [target, ""]
    body = new_body.strip("\n")
    if body:
        new_block.extend(body.splitlines())
    new_block.append("")

    if start_idx == -1:
        if lines and lines[-1].strip() != "":
            lines.append("")
        lines.extend(new_block)
        return "\n".join(lines).rstrip() + "\n"

    end_idx = len(lines)
    for idx in range(start_idx + 1, len(lines)):
        if lines[idx].startswith("## "):
            end_idx = idx
            break

    updated = lines[:start_idx] + new_block + lines[end_idx:]
    return "\n".join(updated).rstrip() + "\n"


def propose(args: dict[str, Any]) -> dict[str, Any]:
    observations = args.get("observations", [])
    if not isinstance(observations, list):
        observations = []
    summarized = [str(item).strip() for item in observations if str(item).strip()]
    if not summarized:
        return {
            "success": True,
            "proposal": "No clear observations were provided. Keep current SOUL directives unchanged.",
        }

    proposal = ["建议更新以下协作偏好（草案）："]
    for item in summarized[:5]:
        proposal.append(f"- {item}")
    proposal.append("建议先更新 `## Collaboration Preferences` 段落并保留回滚点。")
    return {"success": True, "proposal": "\n".join(proposal)}


def apply(args: dict[str, Any]) -> dict[str, Any]:
    path = _resolve_soul_path(args)
    if not _validate_soul_target(path):
        return {"success": False, "error": f"invalid SOUL target path: {path}"}
    if not path.exists():
        return {"success": False, "error": f"SOUL file not found: {path}"}

    immutable_sections = args.get("immutable_sections", _DEFAULT_IMMUTABLE_SECTIONS)
    if not isinstance(immutable_sections, list):
        immutable_sections = _DEFAULT_IMMUTABLE_SECTIONS
    immutable = {str(item).strip() for item in immutable_sections if str(item).strip()}

    changes = args.get("changes", [])
    if not isinstance(changes, list) or not changes:
        return {"success": False, "error": "changes is required"}

    for change in changes:
        section = str(change.get("section", "")).strip()
        if not section:
            return {"success": False, "error": "every change requires section"}
        normalized = section if section.startswith("## ") else f"## {section.lstrip('# ').strip()}"
        if normalized in immutable:
            return {
                "success": False,
                "error": f"section is immutable: {normalized}",
            }

    original = path.read_text(encoding="utf-8")
    checkpoint = _create_checkpoint(path, original)

    updated = original
    updated_sections = []
    for change in changes:
        section = str(change.get("section", "")).strip()
        section_header = section if section.startswith("## ") else f"## {section.lstrip('# ').strip()}"
        new_body = str(change.get("content", "")).strip()
        updated = _replace_section(updated, section_header, new_body)
        updated_sections.append(section_header)

    path.write_text(updated, encoding="utf-8")
    return {
        "success": True,
        "path": str(path),
        "updated_sections": updated_sections,
        "checkpoint": str(checkpoint),
        "event": "workflow.skill.meta.soul_updated",
    }


def list_checkpoints(_: dict[str, Any]) -> dict[str, Any]:
    directory = _checkpoint_dir()
    checkpoints = sorted(str(path) for path in directory.glob("*.md"))
    return {"success": True, "count": len(checkpoints), "checkpoints": checkpoints}


def rollback(args: dict[str, Any]) -> dict[str, Any]:
    checkpoint = str(args.get("checkpoint", "")).strip()
    if not checkpoint:
        return {"success": False, "error": "checkpoint is required"}

    checkpoint_path = Path(checkpoint)
    if not checkpoint_path.is_absolute():
        checkpoint_path = _checkpoint_dir() / checkpoint_path
    if not checkpoint_path.exists():
        return {"success": False, "error": f"checkpoint not found: {checkpoint_path}"}

    path = _resolve_soul_path(args)
    if not _validate_soul_target(path):
        return {"success": False, "error": f"invalid SOUL target path: {path}"}
    if not path.parent.exists():
        path.parent.mkdir(parents=True, exist_ok=True)
    content = checkpoint_path.read_text(encoding="utf-8")
    path.write_text(content, encoding="utf-8")

    return {
        "success": True,
        "path": str(path),
        "checkpoint": str(checkpoint_path),
        "event": "workflow.skill.meta.rollback_applied",
    }


def run(args: dict[str, Any]) -> dict[str, Any]:
    action = args.pop("action", "propose")
    handlers = {
        "propose": propose,
        "apply": apply,
        "list_checkpoints": list_checkpoints,
        "rollback": rollback,
    }
    handler = handlers.get(action)
    if not handler:
        return {"success": False, "error": f"unknown action: {action}"}
    return handler(args)


def main() -> None:
    if len(sys.argv) > 1:
        args = json.loads(sys.argv[1])
    elif not sys.stdin.isatty():
        args = json.load(sys.stdin)
    else:
        args = {}
    result = run(args)
    json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
    sys.stdout.write("\n")
    sys.exit(0 if result.get("success") else 1)


if __name__ == "__main__":
    main()
