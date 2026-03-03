# 2026-03-03 Global Simplify Rescan (Non-web, Wave G)

## Context
Continue non-`web/` simplification after Wave E/F with another subagent-assisted scan, then apply low-risk mechanical cleanup.

## Scope (this wave)
1. Re-scan non-`web/` code for remaining mechanical simplification opportunities.
2. Prioritize `R-01` style normalization (`strings.ToLower(strings.TrimSpace(...))` → `utils.TrimLower(...)`) in runtime-critical CLI and core internal paths.
3. Continue `R-02` style normalization in evaluation flows (`strings.TrimSpace(...) ==/!= ""` → `utils.IsBlank/HasContent`).
4. Keep behavior unchanged; no architectural refactors in this wave.

## Subagent scan summary
- `R-08` (ad-hoc `http.Client`): no remaining high-confidence production bypasses needing immediate non-risky edits in this wave scope.
- `R-06` (manual `ToolResult`): follow-up scan found 3 high-confidence `ApprovalExecutor` returns that can be normalized to `shared.ToolError(...)`.
- Truncation helpers: global unification is still deferred due mixed semantics (rune/byte/domain-specific), but local formatter-level duplication can be safely consolidated first.

## Execution checklist
- [x] Subagent rescan (non-web) completed.
- [x] Apply `TrimLower` normalization batch in `cmd/alex`.
- [x] Apply `TrimLower` normalization batch in selected `internal/domain/agent/react` files.
- [x] Apply `TrimLower` normalization in `internal/infra/attachments/store.go`.
- [x] Apply `TrimLower` normalization batch in `evaluation/agent_eval` + `internal/domain/agent/ports/mocks`.
- [x] Apply `IsBlank/HasContent` normalization batch in `evaluation/agent_eval`.
- [x] Apply `IsBlank/HasContent` normalization batch in selected runtime non-web paths (`internal/app/agent/*`, `internal/delivery/channels/lark`, `internal/domain/agent/react`, `internal/infra/llm`, `internal/devops/process`, `internal/infra/external/teamrun`, `internal/domain/agent/ports`).
- [x] Apply residual non-web mechanical normalization in `internal/{domain/agent/taskfile,devops,delivery/server/bootstrap,infra/tools/builtin/artifacts,shared/config}` and `scripts/memory/backfill_networked.go`.
- [x] Apply subagent-rescan residual normalization in `cmd/alex`, `internal/{app/subscription,app/context,app/agent/preparation,delivery/channels/lark,delivery/server/bootstrap,infra/skills,infra/tools/builtin/larktools,shared/config}`, and `evaluation/agent_eval` (single remaining site).
- [x] Normalize `internal/infra/tools/approval_executor.go` manual error `ToolResult` returns to `shared.ToolError(...)`.
- [x] Consolidate local truncation logic in `internal/delivery/presentation/formatter/formatter.go` to a single rune-safe helper.
- [x] Add shared rune-safe truncation helper in `internal/shared/utils/strings.go` and apply to selected non-web byte-truncation sites (`internal/infra/llm/openai_errors.go`, `internal/delivery/channels/telegram/format.go`, `internal/app/scheduler/notifier.go`, `evaluation/rl/llm_judge.go`).
- [x] Consolidate duplicate `tool executor missing` `ToolResult` construction in `internal/app/toolregistry` via package-local helper.
- [x] Run targeted tests.
- [x] Run targeted lint.
- [x] Run code review gate.

## Verification
- `go test ./cmd/alex ./internal/domain/agent/react ./internal/infra/attachments -count=1`
- `go test ./evaluation/agent_eval ./internal/domain/agent/ports/mocks -count=1`
- `go test ./internal/app/agent/coordinator ./internal/app/agent/kernel ./internal/delivery/channels/lark ./internal/devops/process ./internal/domain/agent/ports ./internal/domain/agent/react ./internal/infra/external/teamrun ./internal/infra/llm -count=1`
- `go test ./internal/domain/agent/taskfile ./internal/devops/... ./internal/delivery/server/bootstrap ./internal/infra/tools/builtin/artifacts ./internal/shared/config ./scripts/memory -count=1`
- `go test ./cmd/alex ./internal/app/agent/coordinator ./internal/app/agent/preparation ./internal/app/context ./internal/app/subscription ./internal/delivery/channels/lark ./internal/delivery/server/bootstrap ./internal/infra/skills ./internal/infra/tools/builtin/larktools ./internal/shared/config ./internal/shared/utils/clilatency ./evaluation/agent_eval -count=1`
- `go test ./internal/infra/tools -count=1`
- `go test ./internal/delivery/presentation/formatter -count=1`
- `go test ./internal/shared/utils ./internal/infra/llm ./internal/delivery/channels/telegram ./internal/app/scheduler ./evaluation/rl -count=1`
- `go test ./internal/app/toolregistry -count=1`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./cmd/alex/... ./internal/domain/agent/react/... ./internal/infra/attachments/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./evaluation/agent_eval/... ./internal/domain/agent/ports/mocks/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./internal/app/agent/coordinator/... ./internal/app/agent/kernel/... ./internal/delivery/channels/lark/... ./internal/devops/process/... ./internal/domain/agent/ports/... ./internal/domain/agent/react/... ./internal/infra/external/teamrun/... ./internal/infra/llm/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./internal/domain/agent/taskfile/... ./internal/devops/... ./internal/delivery/server/bootstrap/... ./internal/infra/tools/builtin/artifacts/... ./internal/shared/config/... ./scripts/memory/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./cmd/alex/... ./internal/app/agent/coordinator/... ./internal/app/agent/preparation/... ./internal/app/context/... ./internal/app/subscription/... ./internal/delivery/channels/lark/... ./internal/delivery/server/bootstrap/... ./internal/infra/skills/... ./internal/infra/tools/builtin/larktools/... ./internal/shared/config/... ./internal/shared/utils/clilatency/... ./evaluation/agent_eval/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./internal/infra/tools/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./internal/delivery/presentation/formatter/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./internal/shared/utils/... ./internal/infra/llm/... ./internal/delivery/channels/telegram/... ./internal/app/scheduler/... ./evaluation/rl/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./internal/app/toolregistry/...`

## Notes
- This wave is intentionally mechanical and scoped to safe replacements only.
- Remaining broader non-`web/` opportunities are now mostly medium/high-risk or require behavior-specific helper APIs.
