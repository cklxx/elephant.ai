# Hooks Integration Test Plan

Date: 2026-03-10

Scope:
- add `internal/runtime/hooks/hooks_integration_test.go`
- cover event bus delivery and stall detector integration behavior

Plan:
1. Inspect the hook event types, bus implementation, and stall detector contracts.
2. Reuse existing runtime integration-test patterns where they already exist.
3. Add integration tests for per-session and wildcard pub/sub delivery.
4. Add integration tests for multi-subscriber fan-out and unsubscribe behavior.
5. Add an integration test that runs the stall detector against a runtime-scanner stub and verifies `EventStalled` delivery through the bus.
6. Run focused integration tests and lint, then mandatory review, commit, merge, and remove the worktree.
