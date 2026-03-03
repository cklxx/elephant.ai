# 2026-03-03 Global Simplify Rescan (Non-web, Wave G)

## Context
Continue non-`web/` simplification after Wave E/F with another subagent-assisted scan, then apply low-risk mechanical cleanup.

## Scope (this wave)
1. Re-scan non-`web/` code for remaining mechanical simplification opportunities.
2. Prioritize `R-01` style normalization (`strings.ToLower(strings.TrimSpace(...))` → `utils.TrimLower(...)`) in runtime-critical CLI and core internal paths.
3. Keep behavior unchanged; no architectural refactors in this wave.

## Subagent scan summary
- `R-08` (ad-hoc `http.Client`): no remaining high-confidence production bypasses needing immediate non-risky edits in this wave scope.
- `R-06` (manual `ToolResult`): remaining candidates are mostly intentional/custom-content paths; defer to a helper-shape cleanup wave.
- Truncation helpers: mixed semantics (rune/byte/domain-specific); defer to dedicated truncation unification wave.

## Execution checklist
- [x] Subagent rescan (non-web) completed.
- [x] Apply `TrimLower` normalization batch in `cmd/alex`.
- [x] Apply `TrimLower` normalization batch in selected `internal/domain/agent/react` files.
- [x] Apply `TrimLower` normalization in `internal/infra/attachments/store.go`.
- [x] Run targeted tests.
- [x] Run targeted lint.
- [x] Run code review gate.

## Verification
- `go test ./cmd/alex ./internal/domain/agent/react ./internal/infra/attachments -count=1`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./cmd/alex/... ./internal/domain/agent/react/... ./internal/infra/attachments/...`

## Notes
- This wave is intentionally mechanical and scoped to safe replacements only.
- Remaining broader non-`web/` opportunities are now mostly medium/high-risk or require behavior-specific helper APIs.
