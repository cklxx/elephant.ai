#!/usr/bin/env python3
"""meta-orchestrator skill — 元技能编排。"""

from __future__ import annotations

from pathlib import Path
import sys

_SCRIPTS_DIR = Path(__file__).resolve().parents[2] / "scripts"
if str(_SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(_SCRIPTS_DIR))

from skill_runner.env import load_repo_dotenv
from skill_runner.cli_contract import parse_cli_args, render_result

load_repo_dotenv(__file__)

from typing import Any
import re


_KAKU_DISPATCH_PATTERNS: tuple[tuple[str, re.Pattern[str]], ...] = (
    ("kaku_cli_dispatch", re.compile(r"\bkaku\s+cli\s+dispatch\b", re.IGNORECASE)),
    ("dispatch_script", re.compile(r"\bscripts/kaku/dispatch\.sh\b", re.IGNORECASE)),
    ("launch_script", re.compile(r"\bscripts/kaku/launch-(?:codex|cc)\.sh\b", re.IGNORECASE)),
    (
        "runtime_session_start",
        re.compile(r"\balex\s+runtime\s+session\s+start\b", re.IGNORECASE),
    ),
)

_KAKU_OBSERVE_PATTERNS: tuple[tuple[str, re.Pattern[str]], ...] = (
    ("kaku_get_text", re.compile(r"\bkaku\s+cli\s+get-text\b", re.IGNORECASE)),
    ("monitor_script", re.compile(r"\bscripts/kaku/monitor\.sh\b", re.IGNORECASE)),
)


def _normalize_governance(level: str) -> str:
    value = (level or "").strip().lower()
    if value in {"low", "medium", "high", "critical"}:
        return value
    return "medium"


def _normalize_mode(mode: str) -> str:
    value = (mode or "").strip().lower()
    if value in {"auto", "semi_auto", "manual"}:
        return value
    return "auto"


def _proactive_cap(level: str) -> int:
    value = (level or "").strip().lower()
    if value == "low":
        return 2
    if value == "high":
        return 5
    return 3


def _highest_risk(skills: list[dict[str, Any]]) -> str:
    rank = {"low": 1, "medium": 2, "high": 3, "critical": 4}
    risk = "low"
    score = 1
    for skill in skills:
        lvl = _normalize_governance(skill.get("governance_level", "medium"))
        if rank[lvl] > score:
            score = rank[lvl]
            risk = lvl
    return risk


def _order_by_dependency(skills: list[dict[str, Any]]) -> list[dict[str, Any]]:
    by_name = {s.get("name", ""): s for s in skills if s.get("name")}
    visited: set[str] = set()
    visiting: set[str] = set()
    ordered: list[dict[str, Any]] = []

    def visit(name: str) -> None:
        if name in visited or name in visiting:
            return
        skill = by_name.get(name)
        if not skill:
            return
        visiting.add(name)
        for dep in skill.get("depends_on_skills", []):
            if dep in by_name:
                visit(dep)
        visiting.remove(name)
        visited.add(name)
        ordered.append(skill)

    for skill in skills:
        name = skill.get("name", "")
        if name:
            visit(name)
    return ordered


def _collect_text_fragments(value: Any) -> list[str]:
    if isinstance(value, str):
        stripped = value.strip()
        return [stripped] if stripped else []
    if isinstance(value, list):
        fragments: list[str] = []
        for item in value:
            fragments.extend(_collect_text_fragments(item))
        return fragments
    if isinstance(value, dict):
        fragments = []
        for item in value.values():
            fragments.extend(_collect_text_fragments(item))
        return fragments
    return []


def _extract_command_context(args: dict[str, Any]) -> list[str]:
    fragments: list[str] = []
    for key, value in args.items():
        if key in {"skills", "links", "plan", "action"}:
            continue
        fragments.extend(_collect_text_fragments(value))
    return fragments


def _detect_kaku_modes(context_fragments: list[str]) -> list[dict[str, Any]]:
    joined = "\n".join(context_fragments)
    detections: list[dict[str, Any]] = []

    dispatch_matches = [
        name for name, pattern in _KAKU_DISPATCH_PATTERNS if pattern.search(joined)
    ]
    if dispatch_matches:
        detections.append(
            {
                "mode": "kaku_dispatch",
                "matched_patterns": dispatch_matches,
                "scheduler_goal": "fan out tasks into dedicated panes with launch wrappers and explicit ownership",
                "recommendations": [
                    "Prefer launch wrappers or runtime session start over raw pane injection so readiness and environment setup stay consistent.",
                    "Assign one pane per agent goal and keep a separate monitor path for status collection before dispatching follow-up work.",
                    "Sequence dependent work explicitly: dispatch implementation panes first, then gate review or follow-up panes on observed completion signals.",
                ],
            }
        )

    observe_matches = [
        name for name, pattern in _KAKU_OBSERVE_PATTERNS if pattern.search(joined)
    ]
    if observe_matches:
        detections.append(
            {
                "mode": "kaku_observe",
                "matched_patterns": observe_matches,
                "scheduler_goal": "treat pane output capture as a scheduling checkpoint, not a fire-and-forget log read",
                "recommendations": [
                    "Read pane output before injecting new work so the scheduler can distinguish idle, working, and done states.",
                    "Use get-text and monitor output to decide whether to continue, wait, or rebalance work to another pane.",
                    "When observation shows completion or shell return, promote downstream tasks or collect artifacts instead of redispatching the same goal.",
                ],
            }
        )

    return detections


def plan(args: dict[str, Any]) -> dict[str, Any]:
    raw_skills = args.get("skills", [])
    if not isinstance(raw_skills, list):
        return {"success": False, "error": "skills must be an array"}

    proactive_level = str(args.get("proactive_level", "medium")).strip().lower() or "medium"
    soul_auto_evolution_enabled = bool(args.get("soul_auto_evolution_enabled", False))

    selected: list[dict[str, Any]] = []
    blocked: list[dict[str, str]] = []
    for item in raw_skills:
        if not isinstance(item, dict):
            continue
        name = str(item.get("name", "")).strip()
        if not name:
            continue

        mode = _normalize_mode(str(item.get("activation_mode", "auto")))
        capabilities = [str(v).strip() for v in item.get("capabilities", []) if str(v).strip()]
        governance_level = _normalize_governance(str(item.get("governance_level", "medium")))
        requires_approval = bool(item.get("requires_approval", False))
        depends = [str(v).strip() for v in item.get("depends_on_skills", []) if str(v).strip()]
        produces_events = [str(v).strip() for v in item.get("produces_events", []) if str(v).strip()]

        if mode == "manual":
            blocked.append({"name": name, "reason": "activation_mode=manual"})
            continue
        if "self_evolve_soul" in {c.lower() for c in capabilities} and not soul_auto_evolution_enabled:
            blocked.append({"name": name, "reason": "soul auto evolution disabled"})
            continue
        if governance_level == "critical" and requires_approval:
            blocked.append({"name": name, "reason": "requires approval"})
            continue

        selected.append(
            {
                "name": name,
                "score": float(item.get("score", 0.0)),
                "governance_level": governance_level,
                "activation_mode": mode,
                "capabilities": capabilities,
                "depends_on_skills": depends,
                "produces_events": produces_events,
            }
        )

    selected.sort(key=lambda x: x.get("score", 0.0), reverse=True)
    cap = _proactive_cap(proactive_level)
    if len(selected) > cap:
        for dropped in selected[cap:]:
            blocked.append({"name": dropped["name"], "reason": "proactive cap reached"})
        selected = selected[:cap]

    selected = _order_by_dependency(selected)

    selected_names = {s["name"] for s in selected}
    links = []
    for link in args.get("links", []) if isinstance(args.get("links", []), list) else []:
        if not isinstance(link, dict):
            continue
        from_name = str(link.get("from", "")).strip()
        to_name = str(link.get("to", "")).strip()
        if from_name in selected_names and to_name in selected_names:
            links.append(
                {
                    "from": from_name,
                    "to": to_name,
                    "on_event": str(link.get("on_event", "")).strip(),
                }
            )

    event_set = set()
    for skill in selected:
        for evt in skill.get("produces_events", []):
            if evt:
                event_set.add(evt)

    context_fragments = _extract_command_context(args)
    scheduling_advice = _detect_kaku_modes(context_fragments)

    return {
        "success": True,
        "selected_skills": [s["name"] for s in selected],
        "blocked_skills": blocked,
        "links": links,
        "risk_level": _highest_risk(selected),
        "proactive_level": proactive_level,
        "events": sorted(event_set),
        "scheduling_advice": scheduling_advice,
    }


def summarize(args: dict[str, Any]) -> dict[str, Any]:
    plan_obj = args.get("plan", {})
    if not isinstance(plan_obj, dict):
        return {"success": False, "error": "plan must be an object"}

    selected = plan_obj.get("selected_skills", [])
    blocked = plan_obj.get("blocked_skills", [])
    links = plan_obj.get("links", [])
    risk = plan_obj.get("risk_level", "low")
    proactive = plan_obj.get("proactive_level", "medium")

    summary = (
        f"Meta Orchestration | proactive={proactive} | risk={risk} | "
        f"selected={len(selected)} | blocked={len(blocked)} | links={len(links)}"
    )
    return {"success": True, "summary": summary}


def run(args: dict[str, Any]) -> dict[str, Any]:
    action = args.pop("action", "plan")
    handlers = {"plan": plan, "summarize": summarize}
    handler = handlers.get(action)
    if not handler:
        return {"success": False, "error": f"unknown action: {action}"}
    return handler(args)


def main() -> None:
    args = parse_cli_args(sys.argv[1:])
    result = run(args)
    stdout_text, stderr_text, exit_code = render_result(result)
    if stdout_text:
        sys.stdout.write(stdout_text)
        if not stdout_text.endswith("\n"):
            sys.stdout.write("\n")
    if stderr_text:
        sys.stderr.write(stderr_text)
        if not stderr_text.endswith("\n"):
            sys.stderr.write("\n")
    sys.exit(exit_code)


if __name__ == "__main__":
    main()
