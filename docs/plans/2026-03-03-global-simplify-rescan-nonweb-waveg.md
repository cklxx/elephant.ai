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
- [x] Apply shared rune-safe truncation helper to additional non-web text-preview paths (`cmd/alex/tui_git.go`, `internal/delivery/channels/lark/rephrase.go`, `internal/infra/lark/summary/group.go`, `internal/delivery/channels/lark/testing/{report,assertions}.go`).
- [x] Apply another residual `TrimLower` normalization pass in non-web runtime paths (`internal/shared/config/runtime_env_loader.go`, `internal/domain/materials/attachment_migrator.go`, `internal/domain/agent/taskfile/resolve.go`, `internal/app/agent/preparation/analysis.go`, `internal/delivery/channels/lark/background_progress_listener.go`).
- [x] Delete dead helper functions in `internal/infra/tools/builtin/orchestration/args.go` flagged by lint (`parseOptionalBool`, `canonicalAgentType`, `isCodingExternalAgent`).
- [x] Normalize additional non-web local truncation helpers to delegate to `utils.TruncateWithSuffix` (`internal/infra/environment/utils.go`, `internal/delivery/channels/lark/chat_context.go`, `cmd/alex/subagent_display.go`, `internal/app/agent/hooks/memory_capture.go`, `internal/app/context/{flush_hook,manager_memory}.go`, `internal/delivery/channels/lark/background_progress_listener.go`).
- [x] Normalize `larktools` and `shared` manual error `ToolResult` construction to shared helpers (`shared.RequireStringArg`, new `larktools/larkToolErrorResult`, `missingChatIDResult`) and fix `shared/context_test.go` nil-context staticcheck warnings.
- [x] Deduplicate `internal/infra/mcp/tool_adapter.go` error `ToolResult` construction and normalize payload preview truncation with `utils.TruncateWithSuffix`.
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
- `go test ./cmd/alex ./internal/delivery/channels/lark ./internal/delivery/channels/lark/testing ./internal/infra/lark/summary -count=1`
- `go test ./internal/shared/config ./internal/domain/materials ./internal/domain/agent/taskfile ./internal/infra/tools/builtin/orchestration ./internal/app/agent/preparation ./internal/delivery/channels/lark -count=1`
- `go test ./cmd/alex ./internal/infra/environment ./internal/delivery/channels/lark ./internal/app/agent/hooks ./internal/app/context -count=1`
- `go test ./internal/infra/tools/builtin/shared ./internal/infra/tools/builtin/larktools ./internal/infra/tools -count=1`
- `go test ./internal/infra/mcp -count=1`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./cmd/alex/... ./internal/domain/agent/react/... ./internal/infra/attachments/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./evaluation/agent_eval/... ./internal/domain/agent/ports/mocks/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./internal/app/agent/coordinator/... ./internal/app/agent/kernel/... ./internal/delivery/channels/lark/... ./internal/devops/process/... ./internal/domain/agent/ports/... ./internal/domain/agent/react/... ./internal/infra/external/teamrun/... ./internal/infra/llm/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./internal/domain/agent/taskfile/... ./internal/devops/... ./internal/delivery/server/bootstrap/... ./internal/infra/tools/builtin/artifacts/... ./internal/shared/config/... ./scripts/memory/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./cmd/alex/... ./internal/app/agent/coordinator/... ./internal/app/agent/preparation/... ./internal/app/context/... ./internal/app/subscription/... ./internal/delivery/channels/lark/... ./internal/delivery/server/bootstrap/... ./internal/infra/skills/... ./internal/infra/tools/builtin/larktools/... ./internal/shared/config/... ./internal/shared/utils/clilatency/... ./evaluation/agent_eval/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./internal/infra/tools/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./internal/delivery/presentation/formatter/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./internal/shared/utils/... ./internal/infra/llm/... ./internal/delivery/channels/telegram/... ./internal/app/scheduler/... ./evaluation/rl/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./internal/app/toolregistry/...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./cmd/alex/... ./internal/delivery/channels/lark/... ./internal/delivery/channels/lark/testing/... ./internal/infra/lark/summary/...`
- `golangci-lint run ./internal/shared/config ./internal/domain/materials ./internal/domain/agent/taskfile ./internal/infra/tools/builtin/orchestration ./internal/app/agent/preparation ./internal/delivery/channels/lark`
- `golangci-lint run ./cmd/alex ./internal/infra/environment ./internal/delivery/channels/lark ./internal/app/agent/hooks ./internal/app/context`
- `golangci-lint run ./internal/infra/tools/builtin/shared ./internal/infra/tools/builtin/larktools ./internal/infra/tools`
- `golangci-lint run ./internal/infra/mcp`

## Notes
- This wave is intentionally mechanical and scoped to safe replacements only.
- Remaining broader non-`web/` opportunities are now mostly medium/high-risk or require behavior-specific helper APIs.
