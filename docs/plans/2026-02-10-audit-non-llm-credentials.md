# Plan: Audit Non-LLM External Credentials

> Created: 2026-02-10
> Status: in-progress
> Trigger: Audit non-LLM external dependencies requiring credentials and produce a module/creds matrix with disable/fallback behavior.

## Goal & Success Criteria
- **Goal**: Identify all non-LLM external dependencies that require separate credentials and classify them as required vs optional for core flows.
- **Done when**: A matrix lists each module/integration, required credentials, whether it can be disabled, and fallback behavior, with sources traced to config/docs/code.
- **Non-goals**: Evaluating LLM provider credentials or changing behavior/configuration.

## Current State
- External integrations span channels, memory stores, observability, and tool integrations across `internal/` and `configs/`.
- Requirements and fallback behavior are likely split across docs, config examples, and implementation defaults.

## Task Breakdown

| # | Task | Files | Size | Depends On |
|---|------|-------|------|------------|
| 1 | Locate integration/config surfaces for channels, memory stores, observability, and integrations | `configs/`, `internal/channels/`, `internal/memory/`, `internal/observability/`, `internal/tools/` | M | — |
| 2 | Extract credential requirements and disable/fallback behavior from config + code | same as T1 plus docs (README/config docs) | M | T1 |
| 3 | Assemble matrix output with required/optional classification and fallback notes | new report output in response | S | T2 |

## Technical Design
- **Approach**: Use targeted ripgrep over configs/docs and key packages to identify credential fields and conditional wiring. Cross-check with code defaults to determine whether missing creds disable modules or degrade to in-memory/no-op behavior. Summarize in a matrix grouped by module.
- **Alternatives rejected**: Rely solely on README/config docs (risk missing code-level fallbacks). Manually read entire codebase (too broad).
- **Key decisions**: Treat “core flows” as baseline local CLI/web runs without optional channels/integrations; mark as required only if the app refuses to start or core flows break without the creds.

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Missing a credential path buried in code | M | M | Use rg on key keywords (`token`, `apikey`, `dsn`, `secret`, `oauth`, `redis`, `postgres`) across integration packages |
| Misclassifying required vs optional | M | H | Confirm with init/wiring paths and defaults; prefer conservative wording when uncertain |

## Verification
- Spot-check each matrix row against at least one config or code reference.
- Verify classifications by identifying disable/enable flags or conditional wiring.
- Rollback: N/A (no code changes expected).
