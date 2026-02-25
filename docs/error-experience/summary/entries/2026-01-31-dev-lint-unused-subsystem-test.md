Summary: `./dev.sh lint` fails due to an unused `(*fakeSubsystem).isStarted` in `internal/server/bootstrap/subsystem_test.go`.
Remediation: Remove the unused helper or assert on it in tests.
