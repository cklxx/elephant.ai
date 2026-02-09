# 2026-02-09 — Baseline Lint/Test Repair and Eval Rerun

Impact: Repository quality baseline recovered (`golangci-lint` + `go test ./...` both green), and expanded foundation suite remained fully stable.

## What changed
- Fixed unchecked error paths (`errcheck`) across devops runtime + supervisor + CLI dev commands.
- Removed restricted `os.Getenv` usages in non-config-managed paths (`cmd/alex/dev*.go`, `internal/devops/services/{backend,sandbox}.go`) by switching to lookup-based access.
- Removed unused field/staticcheck issues and tightened test assertions around file writes.

## Validation
- `./scripts/run-golangci-lint.sh run ./...` ✅
- `scripts/go-with-toolchain.sh test ./...` ✅
- `go run ./cmd/alex eval foundation-suite --suite evaluation/agent_eval/datasets/foundation_eval_suite.yaml --output tmp/foundation-suite-new-cases-20260209 --format markdown` ✅
  - Collections: `17/17`
  - Cases: `408/408`
  - Availability errors: `0`
