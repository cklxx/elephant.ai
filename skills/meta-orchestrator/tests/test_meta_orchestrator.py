"""Tests for meta-orchestrator skill."""

from __future__ import annotations

import importlib.util
from pathlib import Path

_RUN_PATH = Path(__file__).resolve().parent.parent / "run.py"
_spec = importlib.util.spec_from_file_location("meta_orchestrator_run", _RUN_PATH)
_mod = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(_mod)

plan = _mod.plan
summarize = _mod.summarize
run = _mod.run


class TestPlan:
    def test_blocks_manual_and_soul(self):
        result = plan(
            {
                "skills": [
                    {
                        "name": "manual-skill",
                        "activation_mode": "manual",
                        "score": 0.99,
                    },
                    {
                        "name": "soul-self-evolution",
                        "capabilities": ["self_evolve_soul"],
                        "score": 0.9,
                    },
                    {
                        "name": "meta-orchestrator",
                        "score": 0.8,
                    },
                ],
                "soul_auto_evolution_enabled": False,
                "proactive_level": "medium",
            }
        )
        assert result["success"] is True
        assert result["selected_skills"] == ["meta-orchestrator"]
        names = {item["name"] for item in result["blocked_skills"]}
        assert names == {"manual-skill", "soul-self-evolution"}

    def test_dependency_order(self):
        result = plan(
            {
                "skills": [
                    {
                        "name": "child",
                        "score": 0.8,
                        "depends_on_skills": ["parent"],
                    },
                    {
                        "name": "parent",
                        "score": 0.7,
                    },
                ]
            }
        )
        assert result["success"] is True
        assert result["selected_skills"] == ["parent", "child"]


class TestSummarize:
    def test_returns_summary(self):
        result = summarize(
            {
                "plan": {
                    "selected_skills": ["meta-orchestrator"],
                    "blocked_skills": [],
                    "links": [],
                    "risk_level": "high",
                    "proactive_level": "medium",
                }
            }
        )
        assert result["success"] is True
        assert "risk=high" in result["summary"]


class TestRun:
    def test_unknown_action(self):
        result = run({"action": "invalid"})
        assert result["success"] is False
