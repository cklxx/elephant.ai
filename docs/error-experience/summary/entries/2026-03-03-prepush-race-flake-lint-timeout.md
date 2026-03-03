Summary: pre-push 并行门禁在大改动场景下可能出现 `go test -race` 偶发失败和 `golangci-lint --timeout=3m` 超时；需以独立复跑确认真实回归，再决定是否放行。

## Metadata
- id: errsum-2026-03-03-prepush-race-flake-lint-timeout
- tags: [summary, pre-push, race, lint]
- derived_from:
  - docs/error-experience/entries/2026-03-03-prepush-race-flake-lint-timeout.md
