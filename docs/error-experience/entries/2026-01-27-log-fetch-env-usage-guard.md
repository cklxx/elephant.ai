# 2026-01-27 - dev.sh test blocked by env usage guard in log_fetch

## Error
- `./dev.sh test` failed in `internal/config` with: `os.Getenv usage is restricted ... internal/logging/log_fetch.go`.

## Impact
- Full Go test run could not complete.

## Notes / Suspected Causes
- `internal/logging/log_fetch.go` used `os.Getenv` for log directory overrides, which violates the env usage guard.

## Remediation Ideas
- Switch to `os.LookupEnv` or route through config-managed helpers.

## Resolution (This Run)
- Updated `internal/logging/log_fetch.go` to use `os.LookupEnv` for `ALEX_LOG_DIR` and `ALEX_REQUEST_LOG_DIR`.
