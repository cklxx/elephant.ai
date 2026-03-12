## Goal

Quantify stall-recovery effectiveness so the leader can stop repeating low-value `INJECT` strategies.

## Scope

- Add asynchronous recovery evaluation for stall decisions in `internal/runtime/leader`.
- Keep per-session decision history long enough to compute `INJECT` success rate.
- Auto-escalate when repeated `INJECT` decisions stay below a 30% success rate after at least three evaluated attempts.
- Add focused unit tests for recovery success, failure, and low-success auto-escalation.

## Plan

1. Inspect leader stall handling and decision-history lifecycle.
2. Add decision-history helpers for targeted outcome updates and inject success-rate calculation.
3. Update `handleStall` to schedule a non-blocking recovery evaluation, log outcomes, and bypass LLM when inject success is too low.
4. Add targeted tests and run `go test ./internal/runtime/leader/...`, then code review, commit, and fast-forward merge.
