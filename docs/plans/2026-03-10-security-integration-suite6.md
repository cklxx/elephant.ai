# Security Integration Suite 6 Plan

Date: 2026-03-10

Scope:
- add `internal/delivery/server/http/security_integration_test.go`
- cover Suite 6 P0 cases from `output/research/integration-test-plan.md`

Plan:
1. Inspect existing HTTP integration and agent/task execution test harnesses.
2. Build a small router-backed integration fixture in the HTTP package.
3. Implement:
   - `TestSecurity_PathTraversal`
   - `TestSecurity_DataRace`
   - `TestSecurity_LLMAllUnavailable`
4. Run focused integration tests, including a `-race` pass for the data-race scenario.
5. Run code review, commit in worktree, fast-forward merge to `main`, remove worktree.

Progress:
- [x] Inspect existing HTTP integration and agent/task execution test harnesses.
- [x] Confirm router-backed integration fixture in `internal/delivery/server/http/security_integration_test.go`.
- [x] Confirm Suite 6 tests exist and cover path traversal, concurrent task/list/health access, and all-LLM-unavailable failure handling.
- [x] Validate with focused integration tests and `CGO_ENABLED=0 go test -race`.
- [x] Run code review, commit in worktree, fast-forward merge to `main`, remove worktree.

Result:
- `internal/delivery/server/http/security_integration_test.go` was already present on `main` and matches the planned Suite 6 coverage.
- Validation passed on 2026-03-10:
  - `go test -tags=integration ./internal/delivery/server/http -run '^TestSecurity_(PathTraversal|LLMAllUnavailable)$'`
  - `go test -tags=integration ./internal/delivery/server/http -run '^TestSecurity_'`
  - `CGO_ENABLED=0 go test -tags=integration -race ./internal/delivery/server/http -run '^TestSecurity_DataRace$'`
