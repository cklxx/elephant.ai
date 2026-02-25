# Plan: Onboarding/Run Usability Audit

> Created: 2026-02-10
> Status: completed
> Trigger: Audit project usability from onboarding/run perspective (README/docs/Makefile/scripts/docker/.env).

## Goal & Success Criteria
- **Goal**: Produce a concise, file-referenced report of current local run steps, required config/keys, friction points, and quick wins.
- **Done when**: Report lists exact steps and required keys with citations to current repo files, and ranks friction by impact with actionable quick wins.
- **Non-goals**: Editing docs or scripts; changing runtime behavior.

## Current State
- Entry points are README + docs/guides/quickstart.md with additional operational details in deployment and sandbox docs.
- Local run is a mix of `alex dev` flow and legacy `dev.sh`/`deploy.sh` scripts.
- Configuration is YAML-first (`~/.alex/config.yaml`) with env-based secret interpolation and `.env` support for scripts.

## Task Breakdown

| # | Task | Files | Size | Depends On |
|---|------|-------|------|------------|
| 1 | Read onboarding and run docs (README/quickstart/deployment/sandbox) | README.md, docs/guides/quickstart.md, docs/operations/*.md | S | — |
| 2 | Inspect run scripts and examples | Makefile, dev.sh, deploy.sh, scripts/setup_local_auth_db.sh, .env.example | S | T1 |
| 3 | Extract exact local run steps + required keys | examples/config/runtime-config.yaml, docs/reference/CONFIG.md | S | T1, T2 |
| 4 | Identify friction points and quick wins | Findings | S | T1–T3 |
| 5 | Deliver concise report with file references | Output only | S | T4 |

## Technical Design
- **Approach**: Collate run steps from README/quickstart and validate against scripts/Makefile to confirm actual commands and dependencies; extract required keys from runtime-config + .env examples + CONFIG reference; produce ranked friction list and quick wins tied to mismatches or missing guidance.
- **Alternatives rejected**: Running the stack locally (unnecessary for a doc-level audit).
- **Key decisions**: Report will prioritize `alex dev` flow as the recommended path and treat `dev.sh`/`deploy.sh` as secondary/legacy unless explicitly recommended in docs.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Missing a required key or step due to scattered docs | M | H | Cross-check README, quickstart, config example, and scripts before report. |
| Overstating requirements for optional services | M | M | Clearly separate minimal vs optional requirements (auth DB, web UI, Lark). |

## Verification
- Cross-reference all steps with the source files and include file references in the report.
- No code changes; no tests required.
