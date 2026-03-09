# Kernel baseline audit — 20260307T233916Z

## Scope
Validated current canonical package targets after recent Lark path changes.

## Git baseline
- Repo: `/Users/bytedance/code/elephant.ai`
- Branch: `main`
- HEAD: `1dfdbcc580ebe7f7edcd7c0b86135964d81e1b20`
- Upstream: `origin/main`
- Divergence vs upstream: `0 ahead / 0 behind`
- Working tree is **not clean**.
  - Modified: `STATE.md`
  - Modified: `internal/infra/tools/builtin/larktools/docx_manage_test.go`
  - Untracked docs reports/plans already present under `docs/`

## Package validation results
### Go tests
Command:
`go test -count=1 ./internal/infra/teamruntime/... ./internal/app/agent/... ./internal/infra/kernel/... ./internal/infra/lark/...`

Result: **pass**
- `internal/infra/teamruntime/...` ✅
- `internal/app/agent/...` ✅
- `internal/infra/kernel/...` ✅
- `internal/infra/lark/...` ✅

### Lint
Command:
`golangci-lint run ./internal/infra/lark/...`

Result: **pass**

## Larktools baseline check
Path checked:
`./internal/infra/tools/builtin/larktools/...`

Result: **still exists**; not removed.
Representative files confirmed present:
- `channel.go`
- `chat_history.go`
- `docx_manage.go`
- `send_message.go`
- `task_manage.go`
- `wiki_manage.go`

## Canonical baseline targets to keep using
- Test targets:
  - `./internal/infra/teamruntime/...`
  - `./internal/app/agent/...`
  - `./internal/infra/kernel/...`
  - `./internal/infra/lark/...`
- Lint target:
  - `./internal/infra/lark/...`
- Legacy/adjacent path status:
  - `./internal/infra/tools/builtin/larktools/...` remains in tree, so any cleanup/migration work should treat it as live code until explicitly removed.

## Evidence artifacts
- Test output: `/Users/bytedance/.alex/kernel/default/artifacts/kernel-validation-go-test.txt`
- Lint output: `/Users/bytedance/.alex/kernel/default/artifacts/kernel-validation-lark-lint.txt`

