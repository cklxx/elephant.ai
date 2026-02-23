# 2026-02-23 Full Repo Optimization (Subagent Parallel Execution)

## Objective

Execute a repo-wide optimization pass with priority on correctness, security, reliability, and maintainability. The work is implemented in parallel via subagents and validated with full lint + tests before merge.

## Scope

- Backend reliability and concurrency in `internal/`
- LLM routing/streaming/retry robustness in `internal/infra/llm/`
- Tooling and observability hardening in `internal/infra/tools/` + `internal/infra/observability/`
- Frontend security/performance hardening in `web/`
- Script and deploy safety hardening in `scripts/` + `deploy.sh`
- Test/CI alignment updates in `tests/` + `.github/workflows/`

## Best-Practice Baselines

- OWASP ASVS + OWASP Top 10 (XSS/CSRF/command injection/secrets handling)
- Go official guidance for context cancellation, error handling, and bounded memory/concurrency
- POSIX shell secure scripting conventions (no `eval` on untrusted input, checksum verification for downloaded binaries)
- CI reliability norms: deterministic tests, low-flake timing strategy, explicit gates matching documented guarantees

## Execution Plan

1. `in_progress` Fix P0/P1 security risks:
   - iframe sandboxing + URL scheme allowlist
   - command construction injection fixes
   - recursive secret redaction in observability
   - script `eval` removal and supply-chain verification path
2. `pending` Fix P1 reliability bugs:
   - subagent cancellation propagation and bounded parallelism
   - memory pagination cursor correctness
   - LLM fallback filtering and stream robustness
3. `pending` Fix P2/P3 optimization debt:
   - retry-after aware backoff
   - bounded user limiter store
   - large file read streaming and attachment/upload bounds
   - frontend timer cleanup and preview throttling
4. `pending` Test and CI alignment:
   - add/adjust tests for changed logic
   - wire integration-tag checks where feasible
   - reduce flake hotspots where touched
5. `pending` Validation + review:
   - run full lint + tests
   - mandatory code review checklist pass
   - incremental commits and merge back to `main`

## Progress Log

- 2026-02-23 20:23: Initialized plan, loaded engineering practices and latest memory/error-good summaries.
- 2026-02-23 21:02: Completed tools/observability hardening slice: execute_code arg-safe execution, AutoUploadConfig gating for execute_code/shell_exec, recursive nested secret redaction in instrumentation, and large-file range streaming in read_file.
- 2026-02-23 21:03: Added/updated tests for command interpolation safety, upload gating behavior, nested redaction, and large-file invalid range handling; verified with targeted go test runs.
