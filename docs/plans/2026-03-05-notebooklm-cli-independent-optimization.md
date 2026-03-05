# NotebookLM CLI Skill Independent Optimization Plan

Date: 2026-03-05
Owner: codex
Branch: ckl/notebooklm-cli-independent-20260305

## Goal
Make `skills/notebooklm-cli` fully independent and semantically clear for agent use:
- standalone `run.py` wrapper over `nlm`
- explicit, stable parameter schema
- progressive help and concise docs
- complete local tests (unit + executable smoke)

## Steps
1. Baseline & capability check
- verify `nlm` availability and non-destructive help/status commands
- inspect existing skill patterns in repo for runtime/test style

2. Implement standalone runtime
- add `skills/notebooklm-cli/run.py`
- support deterministic actions: `help`, `auth`, `notebook`, `source`, `query`, `report`, `studio`, `raw`
- unify outputs: `{success, command, stdout, stderr, exit_code, hints}`

3. Clarify SKILL docs
- keep progressive disclosure
- add parameter schema and minimal examples mapped to runtime actions

4. Add tests
- add `skills/notebooklm-cli/tests/test_notebooklm_cli.py`
- cover action parsing, safety guardrails, argument requirements, subprocess dispatch

5. Validate end-to-end
- run unit tests for notebooklm skill
- run real `nlm --help`/subcommand help through runtime when CLI exists
- regenerate web skills catalog and verify entry

6. Quality gate
- run mandatory code-review skill
- run targeted lint/test commands
- commit, push, watch CI

## Progress
- [x] Baseline & capability check
  - `nlm` detected: `/Users/bytedance/.local/bin/nlm` (`v0.3.19`)
- [x] Standalone runtime implemented (`skills/notebooklm-cli/run.py`)
  - structured help/schema + progressive disclosure for LLM consumption
  - expanded command options (`profile/json/full/quiet/source_ids/timeout/...`)
  - stronger safety guards (`delete confirm`, `raw interactive block`, parse-error handling)
- [x] SKILL docs rewritten with concise command contract
- [x] Unit tests added/expanded (`skills/notebooklm-cli/tests/test_notebooklm_cli.py`)
  - `18 passed` covering parsing, safety, dispatch, schema help
- [x] Real CLI smoke passed (true network calls)
  - non-destructive matrix: help/auth/notebook/source/query/raw
  - full E2E: notebook create → source add/list → query → report create → studio status → source delete → notebook delete
- [x] Skills catalog regenerated from synced home skills source
- [ ] Final quality gate + commit/push/CI watch
