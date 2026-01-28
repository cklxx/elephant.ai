# 2026-01-28 - dev test request log failures

- Summary: `./dev.sh test` failed because request log files were not found in `web_fetch_test` and `seedream_test`.
- Remediation: verify request log path/async flush; update tests to wait for log queue and/or correct path.
- Status: unresolved in this run.
