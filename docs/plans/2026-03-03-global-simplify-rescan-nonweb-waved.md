# 2026-03-03 Global Simplify Rescan (Non-web, Wave D)

## Context
Continue the global simplify plan in `docs/plans/2026-03-03-global-codebase-simplify.md` with a new non-`web/` worktree and subagent-driven full rescan.

## Scope (this wave)
1. R-06 continuation: migrate another safe batch of manual `ToolResult` error construction to `shared.ToolError(...)`.
2. R-08 continuation: migrate remaining low-risk non-test ad-hoc `&http.Client{}` to `httpclient.New(...)`.
3. CX-07: merge duplicated Codex/Kimi config structs using alias-first rollout (`CLIAgentConfig` and `CLIAgentFileConfig`).

## Subagent rescan outputs used
- R-06 scan: identified safe batch around `larktools/*_manage`, `session/skills`, `orchestration/reply_agent`.
- R-08 scan: identified low-risk direct replacements under `cmd/alex/*` and `evaluation/swe_bench`.
- CX-07/CX-10 scan: recommended alias-first completion for CX-07 in this wave; defer CX-10.
- R-03 scan: no additional low-risk must-do for this wave.

## Selected files
### R-06 slice
- `internal/infra/tools/builtin/session/skills.go`
- `internal/infra/tools/builtin/orchestration/reply_agent.go`
- `internal/infra/tools/builtin/larktools/{bitable_manage,calendar_create,calendar_update,channel,contact_manage,docx_manage,drive_manage,mail_manage,okr_manage,sheets_manage,vc_manage,wiki_manage}.go`

### R-08 slice
- `cmd/alex/cli_model.go`
- `cmd/alex/cli_model_picker.go`
- `cmd/alex/lark_scenario_cmd.go`
- `evaluation/swe_bench/dataset.go`
- `internal/devops/health/checker.go`

### CX-07 slice
- `internal/shared/config/types.go`
- `internal/shared/config/file_config.go`

## Execution checklist
- [x] Apply R-06 replacements in selected files.
- [x] Apply R-08 replacements in selected files with timeout/transport parity.
- [x] Complete CX-07 alias-first refactor.
- [x] Run gofmt on touched files.
- [x] Run targeted tests for touched packages.
- [x] Run full `go test ./...`.
- [x] Run `./scripts/run-golangci-lint.sh run --timeout=10m ./...`.
- [x] Run code review skill.
- [ ] Commit.

## Verification
- `go test ./internal/shared/config/...`
- `go test ./internal/infra/tools/builtin/...`
- `go test ./cmd/alex/...`
- `go test ./internal/devops/health ./evaluation/swe_bench`
- `go test ./...`
- `./scripts/run-golangci-lint.sh run --timeout=10m ./...`

## Execution result
- R-06 reduced remaining manual error-construction matches to 14, now concentrated in:
  - `internal/infra/tools/builtin/larktools/task_manage.go`
  - `internal/infra/tools/builtin/larktools/lark_oauth.go`
  - `internal/infra/tools/builtin/shared/helpers.go` (utility internals).
- R-08 reduced non-test ad-hoc `&http.Client{}` to 3:
  - `internal/infra/acp/client.go` (intentional long-lived/SSE considerations)
  - `internal/domain/materials/attachment_migrator.go` (domain-layer dependency concern)
  - `internal/infra/httpclient/httpclient.go` (factory implementation itself)
- CX-07 completed with alias-first approach:
  - `CodexConfig`/`KimiConfig` unified as `CLIAgentConfig`.
  - `CodexFileConfig`/`KimiFileConfig` unified as `CLIAgentFileConfig`.
