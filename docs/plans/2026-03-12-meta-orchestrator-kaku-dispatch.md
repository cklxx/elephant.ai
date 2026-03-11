# 2026-03-12 meta-orchestrator kaku dispatch

## Goal

Teach `skills/meta-orchestrator/run.py` to detect Kaku CLI dispatch/observation patterns and emit scheduling guidance for those cases.

## Scope

- Inspect the current meta-orchestrator planner and tests.
- Add Kaku CLI pattern recognition for calls such as `kaku cli dispatch` and `kaku cli get-text`.
- Return actionable scheduling guidance without breaking existing planner behavior.
- Validate the script entrypoint after changes.

## Plan

1. Read the current planner logic and tests.
2. Extend the planner with Kaku dispatch/get-text pattern detection and guidance output.
3. Add targeted tests for the new behavior.
4. Run focused tests and `python3 skills/meta-orchestrator/run.py` validation.

## Result

- Added command-context analysis to `skills/meta-orchestrator/run.py` without changing the existing score/dependency selection flow.
- The planner now recognizes Kaku dispatch-related patterns (`dispatch.sh`, launch scripts, runtime session start) and observation patterns (`kaku cli get-text`, `monitor.sh`).
- The plan output now includes `scheduling_advice` with mode-specific scheduler goals and actionable recommendations.

## Validation

- `python3 -m pytest skills/meta-orchestrator/tests/test_meta_orchestrator.py -q`
- `python3 skills/meta-orchestrator/run.py`
- `python3 skills/meta-orchestrator/run.py plan --skills '[{"name":"meta-orchestrator","score":0.8}]' --context 'kaku cli get-text --pane-id 12'`
