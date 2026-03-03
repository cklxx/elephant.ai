# 2026-03-03 Global Simplify Rescan (Non-web, Wave E)

## Context
Continue non-`web/` global simplify after Wave D using subagent rescan, while avoiding unrelated dirty docs changes on `main`.

## Scope (this wave)
1. R-06 continuation: finish remaining low-risk `ToolResult` normalization in `larktools` (`task_manage`, `lark_oauth`).
2. R-08 continuation: address `acp/client` default client with shared HTTP transport while preserving no client-level timeout for SSE.
3. Keep `attachment_migrator` unchanged in this wave due domain-boundary risk; revisit with constructor-contract refactor.
4. Record execution status back to the global simplify plan.

## Execution checklist
- [x] Subagent rescan for R-06 remaining points.
- [x] Subagent rescan for R-08 remaining points.
- [x] Subagent feasibility check for CX-10 (deferred).
- [x] Apply R-06 refactor in `task_manage.go` + `lark_oauth.go`.
- [x] Commit batch-1 (R-06).
- [x] Apply R-08 `acp/client` no-timeout transport refactor.
- [x] Add/adjust tests in `internal/infra/httpclient`.
- [ ] Run targeted/full verification.
- [ ] Update global plan status.
- [ ] Commit batch-2 (R-08 + docs).

## Verification
- `go test ./internal/infra/tools/builtin/larktools -count=1`
- `go test ./internal/infra/httpclient ./internal/infra/acp -count=1`
- `go test ./...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./...`

## Notes
- `CX-10` remains deferred this wave (medium risk: YAML inline compatibility + Telegram coverage gap).
