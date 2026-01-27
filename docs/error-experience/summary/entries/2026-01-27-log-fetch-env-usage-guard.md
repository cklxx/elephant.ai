# 2026-01-27 - dev.sh test blocked by env usage guard in log_fetch

- Summary: `./dev.sh test` failed because `internal/logging/log_fetch.go` used `os.Getenv`.
- Remediation: switch to `os.LookupEnv` or config-managed access.
- Resolution: updated to `os.LookupEnv` in this run.
