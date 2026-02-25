# 2026-01-31 - dev.sh lint unused helper in subsystem_test

## Error
- `./dev.sh lint` failed: `internal/server/bootstrap/subsystem_test.go:41:25` unused method `(*fakeSubsystem).isStarted`.

## Impact
- Full lint step blocked.

## Notes
- Not related to the internal/agent refactor.

## Remediation Ideas
- Remove the unused method or assert on it in tests.

## Status
- observed
