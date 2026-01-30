# 2026-01-30 - dev.sh lint fails on errcheck in tests

## Error
- `./dev.sh lint` fails with errcheck findings:
  - `internal/agent/app/hooks/memory_capture_test.go:176:22` missing error check for `hook.OnTaskCompleted`.
  - `internal/agent/app/hooks/integration_test.go:196:10` missing error check for `svc.Save`.

## Impact
- Full lint validation fails; changes cannot be fully validated with `./dev.sh lint`.

## Notes / Suspected Causes
- Pre-existing errcheck violations in test files; not related to the background task changes.

## Remediation Ideas
- Update the tests to assert/handle returned errors.

## Resolution
- Added error checks in `internal/agent/app/hooks/memory_capture_test.go` and `internal/agent/app/hooks/integration_test.go`.
- `./dev.sh lint` passes.
