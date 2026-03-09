# Kernel Baseline Audit — 2026-03-06T04:40:18Z

## Scope
Independent audit run in `/Users/bytedance/code/elephant.ai` to refresh a verifiable kernel baseline and provide parallel validation reference for upcoming build-executor fixes. No build logic changes were made.

## Repository Baseline
- Repo: `/Users/bytedance/code/elephant.ai`
- Host: `Darwin 25.3.0 arm64`
- Go: `go1.26.0 darwin/arm64`
- ripgrep: `15.1.0`
- Commit under audit: `2c9bad23`
- Git status at start/end:
  - `## main...origin/main [ahead 1]`
  - Modified: `STATE.md`
  - Modified: `internal/infra/tools/builtin/larktools/docx_manage_test.go`
  - Untracked before this run: existing docs reports/plans files

## Verified package paths
Commands executed:
- `go list ./... | rg 'lark|larktools'`
- `go list ./internal/infra/teamruntime/... ./internal/app/agent/... ./internal/infra/kernel/...`
- `rg -n 'package (lark|larktools)' internal`

Confirmed relevant packages:
- `alex/internal/infra/teamruntime`
- `alex/internal/app/agent/config`
- `alex/internal/app/agent/context`
- `alex/internal/app/agent/coordinator`
- `alex/internal/app/agent/cost`
- `alex/internal/app/agent/hooks`
- `alex/internal/app/agent/kernel`
- `alex/internal/app/agent/llmclient`
- `alex/internal/app/agent/preparation`
- `alex/internal/infra/kernel`
- `alex/internal/infra/lark`
- `alex/internal/infra/lark/calendar/meetingprep`
- `alex/internal/infra/lark/calendar/suggestions`
- `alex/internal/infra/lark/oauth`
- `alex/internal/infra/lark/summary`
- `alex/internal/infra/tools/builtin/larktools`
- `alex/internal/delivery/channels/lark`
- `alex/internal/delivery/channels/lark/testing`

Decision: treat `internal/infra/lark`, `internal/infra/tools/builtin/larktools`, and `internal/delivery/channels/lark` as the active Lark-related verification set for this audit.

## Deterministic baseline tests
Executed:
```bash
go test ./internal/infra/teamruntime/... ./internal/app/agent/... ./internal/infra/kernel/...
go test ./internal/infra/lark/... ./internal/infra/tools/builtin/larktools ./internal/delivery/channels/lark/...
```

Results:
- Core/kernel set: PASS
  - `alex/internal/infra/teamruntime` — `ok` (`14.586s`)
  - `alex/internal/app/agent/config` — `ok`
  - `alex/internal/app/agent/context` — `ok`
  - `alex/internal/app/agent/coordinator` — `ok` (`1.185s`)
  - `alex/internal/app/agent/cost` — `ok` (`1.981s`)
  - `alex/internal/app/agent/hooks` — `ok` (`2.376s`)
  - `alex/internal/app/agent/kernel` — `ok` (`6.813s`)
  - `alex/internal/app/agent/llmclient` — `ok`
  - `alex/internal/app/agent/preparation` — `ok` (`1.557s`)
  - `alex/internal/infra/kernel` — `ok`
- Lark/Larktools set: PASS
  - `alex/internal/infra/lark` — `ok`
  - `alex/internal/infra/lark/calendar/meetingprep` — `ok`
  - `alex/internal/infra/lark/calendar/suggestions` — `ok`
  - `alex/internal/infra/lark/oauth` — `ok`
  - `alex/internal/infra/lark/summary` — `ok`
  - `alex/internal/infra/tools/builtin/larktools` — `ok` (`2.168s`)
  - `alex/internal/delivery/channels/lark` — `ok` (`53.006s`)
  - `alex/internal/delivery/channels/lark/testing` — `ok` (`4.036s`)

Raw logs:
- `/tmp/kernel_audit_test_core.log`
- `/tmp/kernel_audit_test_lark.log`

## Minimal lint
Executed:
```bash
/Users/bytedance/go/bin/golangci-lint run \
  ./internal/infra/teamruntime/... \
  ./internal/app/agent/... \
  ./internal/infra/kernel/... \
  ./internal/infra/lark/... \
  ./internal/infra/tools/builtin/larktools \
  ./internal/delivery/channels/lark/...
```

Result:
- PASS, no findings emitted
- Lint version: `golangci-lint v1.64.8`
- Raw log: `/tmp/kernel_audit_lint.log` (empty because command succeeded with zero findings)

## Notes for STATE.md
Suggested key points to append/update (without editing build logic):
- `2026-03-06T04:40:18Z kernel baseline audit completed on commit 2c9bad23.`
- `Verified deterministic green baseline for teamruntime, app/agent, infra/kernel, infra/lark, delivery/channels/lark, and infra/tools/builtin/larktools.`
- `Confirmed active Lark-related packages: internal/infra/lark, internal/infra/tools/builtin/larktools, internal/delivery/channels/lark.`
- `golangci-lint v1.64.8 on the same package set returned zero findings.`
- `Working tree was not clean during audit: STATE.md and internal/infra/tools/builtin/larktools/docx_manage_test.go modified; existing untracked docs report/plan files present.`

## Risk / follow-up
- Baseline is green for the audited package set, so future build-executor failures are more likely to be introduced by packaging/selection logic, environment drift, or newly touched files outside this set.
- Since the tree was already dirty, future repair work should preserve unrelated local changes and compare against commit `2c9bad23` plus this report.

