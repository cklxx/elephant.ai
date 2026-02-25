# Plan: Auto-load .env for All Skill Scripts (2026-02-10)

## Goal
Ensure every `skills/*/run.py` automatically loads repository `.env` before reading environment variables, so direct skill execution works without manual `source .env`.

## Scope
- Add a shared Python helper for `.env` loading.
- Wire helper into every skill script entry (`skills/*/run.py`).
- Update skill scaffold template to include the same auto-load behavior for newly created skills.
- Add/adjust tests for helper behavior and run full Python skill test suite.

## Out of Scope
- Changing existing skill business behavior beyond env loading.
- Changing Go runtime config loading logic.

## Plan
1. [x] Add shared helper module for dotenv loading using `python-dotenv` (with auto-install fallback) and deterministic precedence (`override=False` default).
2. [x] Integrate helper in all `skills/*/run.py` files.
3. [x] Update `skills/auto-skill-creation/run.py` template output to include the same helper wiring.
4. [x] Add tests for dotenv helper and run full Python skill tests.
5. [x] Run lint/tests and verify image-creation can read env without manual sourcing.

## Risks
- Batch edits may accidentally break import order or template string escaping.
- Existing tests relying on clean env may become flaky.

## Mitigations
- Use a deterministic insertion pattern and verify every changed file compiles via pytest collection.
- Keep helper non-overriding by default.
- Add targeted helper unit tests for precedence and parsing.

## Progress Log
- 2026-02-10 17:10: Created plan and confirmed 30 skill `run.py` entry scripts require unified env auto-loading.
- 2026-02-10 17:16: Switched to `python-dotenv`-based helper with runtime auto-install (`pip install python-dotenv`) when missing.
- 2026-02-10 17:18: Wired helper into all 30 `skills/*/run.py` entries and updated auto-skill scaffold template output.
- 2026-02-10 17:20: Added helper tests (`scripts/skill_runner/tests/test_env.py`), ran full validation: lint + `make test` + python tests all green.
