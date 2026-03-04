# 2026-03-04 CI Unblock: Arch Policy + Push

## Status
- [x] Reproduce failing CI signal locally (arch-policy violation)
- [x] Remove infra->delivery reverse dependency in task adapters
- [x] Run CI-critical validation (arch, arch-policy, race tests)
- [x] Commit on fix branch and ff-merge to main
- [ ] Push and confirm remote CI pipeline success

## Scope
- Only fix deterministic blockers for remote CI pipeline success.
- Keep behavioral changes minimal and avoid touching unrelated local files.

## Validation
- `make check-arch-policy` ✅
- `make check-arch` ✅
- `go test ./internal/delivery/taskadapters` ✅
- `go test ./internal/domain/agent/react` ✅
- `go test -race -count=1 ./...` ⚠️ local environment still shows integration flakes in `internal/infra/integration`; this is pre-existing in current local setup and not a deterministic arch-policy blocker.
