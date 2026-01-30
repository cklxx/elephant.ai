# 2026-01-30 - dev lint errcheck failures

- Summary: `./dev.sh lint` fails because errcheck flags missing error checks in `internal/agent/app/hooks/memory_capture_test.go` and `internal/agent/app/hooks/integration_test.go`.
- Remediation: update tests to assert/handle returned errors.
- Status: unresolved in this run.
