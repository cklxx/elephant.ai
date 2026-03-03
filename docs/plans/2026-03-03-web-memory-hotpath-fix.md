# 2026-03-03 Web Memory Hotpath Fix

## Goal
Reduce `alex-web` peak memory during active task execution by removing heavy full-payload reads on high-frequency polling endpoints.

## Scope
- `GET /api/sessions`
- `GET /api/sessions/{session_id}/snapshots`

## Plan
1. Add a lightweight session list path that reads session headers without loading full message payloads.
2. Change snapshot metadata listing to avoid loading full snapshot payloads.
3. Update API handler list path to use lightweight data flow.
4. Add/adjust tests and run verification.

## Progress
- [x] Baseline root-cause confirmed with runtime/process/log evidence.
- [x] Implement lightweight session list path.
- [x] Implement metadata-only snapshot listing path.
- [x] Validate with focused tests and full checks.

## Validation
- `go test ./internal/infra/session/filestore ./internal/infra/session/state_store ./internal/delivery/server/http ./internal/delivery/server/app` ✅
- `go test ./...` ✅
- `./scripts/run-golangci-lint.sh run` ✅
