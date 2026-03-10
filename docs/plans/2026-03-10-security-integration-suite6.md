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
