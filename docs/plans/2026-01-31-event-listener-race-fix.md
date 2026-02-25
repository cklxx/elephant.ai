# Event Listener Race Fix Plan

**Goal:** Eliminate the send/close data race in `SerializingEventListener` and stabilize coordinator tests under `-race` without changing production semantics.

**Context:** `go test -race -covermode=atomic ./internal/agent/app/coordinator` intermittently reports a race between `eventQueue.close()` (closing the channel) and `OnEvent` (sending), triggered by the per-run goroutines in `SerializingEventListener`.

## Plan
1. **Fix the queue shutdown protocol**
   - Replace channel closing with a `done` channel signal.
   - Ensure `OnEvent` checks `done` and uses `select` to avoid sending after shutdown.
   - Keep per-run ordering and idle cleanup unchanged.
2. **Make test helpers concurrency-safe**
   - Add mutexes to `stubSessionStore` and `stubHistoryManager`.
   - Keep test listeners locked when recording events.
3. **Align tests with updated behavior**
   - Set `UserID` in memory capture save-error test to exercise the error path.
4. **Validation**
   - `go test -race -covermode=atomic -coverprofile=/tmp/coordinator_cover.out ./internal/agent/app/coordinator -count=1`
   - `./dev.sh lint`
   - `./dev.sh test`

## Status
- [x] Implement queue shutdown via `done` channel.
- [x] Add mutexes to coordinator test stubs/listeners.
- [x] Update memory capture error test.
- [x] Run full lint + tests.
