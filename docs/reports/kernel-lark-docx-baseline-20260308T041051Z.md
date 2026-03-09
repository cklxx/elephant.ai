# Kernel Lark/Docx Focused Baseline Report

- Timestamp (UTC): 20260308T041051Z
- Repo: /Users/bytedance/code/elephant.ai
- HEAD: 1dfdbcc580ebe7f7edcd7c0b86135964d81e1b20
- larktools path: present

## Git Status

```
 M STATE.md
 M internal/infra/tools/builtin/larktools/docx_manage_test.go
?? docs/plans/2026-03-06-agent-team-feishu-cli-terminal-integration.md
?? docs/reports/autonomous-audit-docx-lark-boundary-20260308T003921Z.md
?? docs/reports/autonomous-audit-docx-lark-boundary-20260308T004141Z.md
?? docs/reports/build-executor-adjacent-autonomous-audit-20260308T000925Z.md
?? docs/reports/kernel-autonomous-audit-lark-docx-larktools-20260308.md
?? docs/reports/kernel-autonomous-verification-20260308T010916Z.md
?? docs/reports/kernel-autonomous-verification-20260308T011056Z.md
?? docs/reports/kernel-autonomous-verification-20260308T011118Z.md
?? docs/reports/kernel-baseline-audit-20260306T044018Z.md
?? docs/reports/kernel-baseline-audit-20260306T044326Z.md
?? docs/reports/kernel-baseline-audit-20260306T051004Z.md
?? docs/reports/kernel-baseline-audit-20260307T233916Z.md
?? docs/reports/kernel-baseline-audit-20260307T234043Z.md
?? docs/reports/kernel-cycle-2026-03-06T03-08Z.md
?? docs/reports/kernel-cycle-2026-03-06T04-08Z-lark-docx-convert-revalidation.md
?? docs/reports/kernel-focused-build-verification-20260308T020935Z.md
?? docs/reports/kernel-lark-docx-baseline-20260308T040914Z.md
?? docs/reports/larktools-lint-backlog-audit-20260306T040917Z.md
```

## Commands Run

```bash
go test -count=1 ./internal/infra/lark/... ./internal/infra/teamruntime/... ./internal/app/agent/... ./internal/infra/kernel/...
golangci-lint run ./internal/infra/lark/...

golangci-lint run ./internal/infra/tools/builtin/larktools/...
```

## go test Output

```
ok  	alex/internal/infra/lark	0.820s
ok  	alex/internal/infra/lark/calendar/meetingprep	1.159s
ok  	alex/internal/infra/lark/calendar/suggestions	0.792s
ok  	alex/internal/infra/lark/oauth	1.602s
ok  	alex/internal/infra/lark/summary	1.673s
ok  	alex/internal/infra/teamruntime	19.705s
ok  	alex/internal/app/agent/config	1.921s
ok  	alex/internal/app/agent/context	2.237s
ok  	alex/internal/app/agent/coordinator	2.700s
ok  	alex/internal/app/agent/cost	3.204s
ok  	alex/internal/app/agent/hooks	3.519s
ok  	alex/internal/app/agent/kernel	8.398s
ok  	alex/internal/app/agent/llmclient	2.495s
ok  	alex/internal/app/agent/preparation	3.401s
ok  	alex/internal/infra/kernel	3.728s
```

## golangci-lint Output: internal/infra/lark

```

```

## golangci-lint Output: internal/infra/tools/builtin/larktools

```

```

## Conclusion

### Currently stable packages
- Packages under `./internal/infra/lark/...` passed `go test` in this run.
- Packages under `./internal/infra/teamruntime/...` passed `go test` in this run.
- Packages under `./internal/app/agent/...` passed `go test` in this run.
- Packages under `./internal/infra/kernel/...` passed `go test` in this run.
- Scoped lint for `./internal/infra/lark/...` passed in this run.
- Scoped lint for `./internal/infra/tools/builtin/larktools/...` passed in this run.

### Remaining lint/test risks
- This is a focused baseline only; full-repo test and lint coverage remains unverified.
- Results are tied to the current working tree state, including uncommitted changes.
- Other packages outside the target surface may still carry regressions or lint debt.

### Suggested next step
- Run full-repo `go test ./...` and broader `golangci-lint run ./...` if you want a release-grade confidence check.
- If the active work stays in lark/docx, keep using this focused suite as the fast regression gate before widening scope.
