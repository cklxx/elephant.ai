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

1. `completed` Fix P0/P1 security risks:
   - iframe sandboxing + URL scheme allowlist
   - command construction injection fixes
   - recursive secret redaction in observability
   - script `eval` removal and supply-chain verification path
2. `completed` Fix P1 reliability bugs:
   - subagent cancellation propagation and bounded parallelism
   - memory pagination cursor correctness
   - LLM fallback filtering and stream robustness
3. `completed` Fix P2/P3 optimization debt:
   - retry-after aware backoff
   - large file read streaming and attachment/upload bounds
   - frontend timer cleanup and preview throttling
4. `completed` Test and CI alignment:
   - add/adjust tests for changed logic
   - run repository CI-equivalent gates (`go vet/build/test -race/lint/arch`)
   - run explicit `web lint + build` validation
5. `completed` Validation + review:
   - run full lint + tests
   - mandatory code review checklist pass
   - incremental commits and merge back to `main`

## Progress Log

- 2026-02-23 20:23: Initialized plan, loaded engineering practices and latest memory/error-good summaries.
- 2026-02-23 20:35: Completed parallel subagent implementation across web/internal/tools/scripts and passed targeted package tests.
- 2026-02-23 20:43: Passed full pre-push checks (`go vet/build/test -race/lint/arch`) and explicit `web lint + build`.
- 2026-02-23 20:49: Closed remaining hook-command quoting risk in Lark CC hooks and added regression coverage.
- 2026-02-23 21:02: Completed tools/observability hardening slice: execute_code arg-safe execution, AutoUploadConfig gating for execute_code/shell_exec, recursive nested secret redaction in instrumentation, and large-file range streaming in read_file.
- 2026-02-23 21:03: Added/updated tests for command interpolation safety, upload gating behavior, nested redaction, and large-file invalid range handling; verified with targeted go test runs.
- 2026-02-23 21:16: Rebased onto latest `main`, reran full pre-push gates (including web checks), fast-forward merged back to `main`, and removed temporary worktree/branch.
