# Acceptance (Deprecated Shell Chain)

The legacy shell acceptance chain under `tests/acceptance/*.sh` was removed on 2026-02-12.

Reason:
- It no longer matches current server contracts:
- `/health` payload changed
- API endpoints now require authentication
- task creation response now uses `run_id` (not `task_id`)

Current validation path:
- Full regression: `go test ./... -count=1`
- Lint: `./scripts/run-golangci-lint.sh run ./...`
- Lark scenarios: `go run ./cmd/alex lark scenario run --dir tests/scenarios/lark`

For end-to-end server verification, use authenticated API flows against `alex-web`
instead of the removed unauthenticated shell scripts.
