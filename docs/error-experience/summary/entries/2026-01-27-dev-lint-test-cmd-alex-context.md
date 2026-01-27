# 2026-01-27 - dev lint/test blocked by cmd/alex context errors

- Summary: `./dev.sh lint` and `./dev.sh test` failed because `cmd/alex/cost.go` references `context` without import and `cmd/alex/acp.go` imports `context` unused.
- Remediation: add/remove the `context` imports or wire `context` usage correctly, then rerun lint/test.
- Resolution: none in this run (doc-only changes).
