# 2026-03-03 Global Simplify Rescan (Non-web, Wave C)

## Context
Based on `docs/plans/2026-03-03-global-codebase-simplify.md`, run a subagent-driven global rescan excluding `web/` and continue low-risk backend optimizations.

## Scope (this wave)
1. R-06 continuation: replace manual `ToolResult` error construction with `shared.ToolError()` in a selected non-web slice.
2. R-08 continuation: replace safe ad-hoc `&http.Client{}` with `httpclient.New()` while preserving timeout semantics.
3. CX-09: extract shared OpenAI request-building helper used by both `Complete` and `StreamComplete`.

## Selected files
### R-06 slice
- `internal/infra/tools/builtin/aliases/read_file.go`
- `internal/infra/tools/builtin/aliases/write_file.go`
- `internal/infra/tools/builtin/aliases/shell_exec.go`
- `internal/infra/tools/builtin/aliases/replace_in_file.go`
- `internal/infra/tools/builtin/larktools/upload_file.go`
- `internal/infra/tools/builtin/larktools/calendar_query.go`

### R-08 slice
- `internal/infra/llamacpp/downloader.go`
- `internal/shared/config/cli_auth.go`
- `internal/shared/modelregistry/registry.go`
- `internal/delivery/channels/lark/model_command.go`
- `internal/app/notification/notification.go`

### CX-09 slice
- `internal/infra/llm/openai_client.go`
- `internal/infra/llm/openai_client_test.go` (if needed)

## Execution checklist
- [x] Apply R-06 replacements in selected files.
- [x] Apply R-08 replacements in selected files.
- [x] Implement CX-09 helper extraction and keep behavior parity.
- [x] Run gofmt on touched files.
- [x] Run targeted tests for touched packages.
- [x] Update `docs/plans/2026-03-03-global-codebase-simplify.md` execution status.
- [x] Run code review skill.
- [ ] Commit.

## Verification
- `go test ./internal/infra/tools/builtin/aliases ./internal/infra/tools/builtin/larktools -count=1`
- `go test ./internal/infra/llamacpp ./internal/shared/config ./internal/shared/modelregistry ./internal/app/notification ./internal/delivery/channels/lark -count=1`
- `go test ./internal/infra/llm -count=1`

## Execution result
- R-06 slice completed across 6 files (aliases + larktools), preserving context-rich error messages while standardizing on `shared.ToolError(...)`.
- R-08 slice completed across 5 files with timeout-compatible `httpclient.New(...)` replacement.
- CX-09 completed by extracting `buildOpenAIRequest(req, stream bool)` and adding request-equivalence tests in `openai_client_test.go`.
