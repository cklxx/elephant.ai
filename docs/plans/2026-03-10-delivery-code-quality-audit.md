# 2026-03-10 Delivery Code Quality Audit

## Goal

Audit `internal/delivery/` for:

- dead code
- unused handlers
- stale route registration
- overlong functions that can be simplified safely

## Checklist

- [x] Create worktree and marker.
- [x] Inspect routes, handlers, and function complexity.
- [x] Remove dead code and simplify functions.
- [x] Run relevant validation.
- [x] Run code review.
- [x] Commit, merge, and push.

## Notes

- Removed stale method-dispatch entrypoints that were no longer used by route registration: `AppsConfigHandler.HandleAppsConfig` and `ContextConfigHandler.HandleContextConfig`.
- Simplified route registration by extracting grouped helpers out of `router.go` and `router_debug.go`.
- `git diff --check` passed.
- `go test ./internal/delivery/server/http ./internal/delivery/server/bootstrap -count=1` passed.
- `python3 skills/code-review/run.py review` ran successfully but returned raw diff payload instead of structured findings.
- `scripts/pre-push.sh` passed.
