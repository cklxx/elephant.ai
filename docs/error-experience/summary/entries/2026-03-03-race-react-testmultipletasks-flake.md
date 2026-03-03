Summary: `go test -race -count=1 ./...` 在高负载全量执行时，`internal/domain/agent/react TestMultipleTasks` 出现一次 completion 计数抖动；定向复跑通过，说明当前更像 flaky 而非确定性回归。

## Metadata
- id: errsum-2026-03-03-race-react-testmultipletasks-flake
- tags: [summary, race, flaky-test, react]
- derived_from:
  - docs/error-experience/entries/2026-03-03-race-react-testmultipletasks-flake.md
