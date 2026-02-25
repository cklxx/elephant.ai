# 2026-02-25 - Lark loop fast-gate recovery via config-test hermeticity

## Background
- Incident: `afx-20260225T062054Z-kernel-d001186b`
- Symptom in loop fast gate: config tests fail with `invalid llm profile ... sk-kimi ... base_url=https://api.openai.com/v1`.
- Root cause from logs/code: host environment key (`OPENAI_API_KEY=sk-kimi-*`) leaks into tests that expect missing-key behavior and do not clear env.

## Goals
1. Restore fast-gate stability for `main=d001186b` without relaxing runtime profile validation.
2. Keep change scope minimal and maintainable.
3. Leave repository committable after validation.

## Plan
- [x] Confirm failure signature and isolate affected tests.
- [x] Make failing tests hermetic by clearing validation-sensitive env vars.
- [x] Run focused tests for previously failing packages.
- [x] Run fast-gate equivalent validation (`scenario run` + `go test ./... -count=1 -p 2` in worktree).
- [x] Summarize results and residual risk.

## Progress log
- 2026-02-25: Identified missing env cleanup in `cmd/alex/config_test.go` and `internal/delivery/server/bootstrap/config_test.go` quickstart case.
- 2026-02-25: Applied minimal test-only changes to clear inherited LLM env vars before config load assertions.
- 2026-02-25: `CGO_ENABLED=0 go test ./cmd/alex -count=1` passed.
- 2026-02-25: `CGO_ENABLED=0 go test ./internal/delivery/server/bootstrap -count=1` passed.
- 2026-02-25: Fast-gate equivalent passed in worktree:
  - `go run ./cmd/alex lark scenario run --mode mock --dir tests/scenarios/lark`
  - `CGO_ENABLED=0 go test ./... -count=1 -p 2`
- 2026-02-25: `./dev.sh lint` blocked by local environment dependency (`web` lint: `eslint: command not found`).
