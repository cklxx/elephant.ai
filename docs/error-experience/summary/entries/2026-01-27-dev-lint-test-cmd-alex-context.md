# 2026-01-27 - dev lint/test blocked by cmd/alex context errors

- Summary: `./dev.sh lint` and `./dev.sh test` failed because `cmd/alex/cost.go` referenced `context` without import and `cmd/alex/acp.go` imported `context` unused.
- Remediation: adjust the `context` imports to match usage and rerun lint/test.
- Resolution: fixed imports and reran lint/test successfully in this run.
