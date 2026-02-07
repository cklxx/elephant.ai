# Plan: Fix Lark Card Callbacks + Log Cleanup

**Status:** COMPLETED
**Date:** 2026-02-08
**Branch:** fix/lark-card-callback-log-cleanup

## Summary

Fixed two categories of issues:
1. **Lark card buttons not working** in standalone WS mode — SDK v3.5.3 silently drops MessageTypeCard frames
2. **Log file noise and growth** — DEBUG default, no rotation, supervisor SHA drift

## Commits

| # | Hash | Message |
|---|------|---------|
| 1 | edff12fe | feat(config): add card_callback_port to Lark config |
| 2 | 036b0288 | feat(lark): add card callback HTTP server in standalone mode |
| 3 | 4c881c3d | fix(logger): default log level to INFO, configurable via ALEX_LOG_LEVEL |
| 4 | 9137269e | refactor(config_log): condense startup config logging |
| 5 | 4d20c1d0 | feat(logger): add log rotation via lumberjack |
| 6 | 6e595db4 | fix(build): exclude volatile dirs from build fingerprint |

## Key Decisions

- Card callback HTTP server runs on port 9292 (configurable), reuses existing `NewCardCallbackHandler`
- Logger default changed from DEBUG → INFO; configurable via `ALEX_LOG_LEVEL` env var
- Log rotation: 100MB max size, 3 backups, 7 days, compressed (via lumberjack v2.2.1)
- Build fingerprint excludes: logs/, .pids/, eval-server/, .worktrees/

## Verification

- All tests pass: `go test ./internal/shared/utils/... ./internal/delivery/server/bootstrap/... ./internal/delivery/channels/lark/...`
- New tests added for config parsing, env fallback, log level resolution, and max size resolution
