# Hooks Integration Test Plan

Date: 2026-03-10

Scope:
- add `internal/runtime/hooks/hooks_integration_test.go`
- cover event bus delivery and stall detector integration behavior

Plan:
- [x] Inspect the hook event types, bus implementation, and stall detector contracts.
- [x] Reuse existing runtime integration-test patterns where they already exist.
- [x] Add integration tests for per-session and wildcard pub/sub delivery.
- [x] Add integration tests for multi-subscriber fan-out and unsubscribe behavior.
- [x] Add an integration test that runs the stall detector against a runtime-scanner stub and verifies `EventStalled` delivery through the bus.
- [x] Run focused integration tests and lint, then mandatory review, commit, merge, and remove the worktree.

## Added tests (6 new, extending existing 4)
- `TestEventBus_SessionIsolation` — publish to session-A does NOT reach session-B subscriber
- `TestEventBus_WildcardUnsubscribe` — SubscribeAll cancel prevents delivery
- `TestEventBus_MultipleWildcardSubscribers` — fan-out to 2 wildcard subscribers
- `TestEventBus_PayloadDelivery` — event payload round-trips correctly
- `TestStallDetector_MultipleStalledSessions` — detector emits EventStalled for 2 concurrent stalled sessions
