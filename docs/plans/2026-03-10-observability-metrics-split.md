Status: completed

Objective: split `internal/infra/observability/metrics.go` into smaller focused files without changing any exported surface or behavior.

Steps:
1. Inspect `metrics.go` and neighboring observability tests/usages to group responsibilities.
2. Move the code into focused files under `internal/infra/observability`, keeping function signatures and exports unchanged.
3. Run targeted review plus `go build` and `go test` to verify the package and repo still compile and pass.

Outcome:
- Split `metrics.go` into constructor/shared state plus focused `metrics_llm.go`, `metrics_http.go`, `metrics_system.go`, and `metrics_leader.go`.
- Kept all existing exported signatures and behavior intact.
- Verified with `go build ./...`, `go test ./...`, and the standard code-review pass.
