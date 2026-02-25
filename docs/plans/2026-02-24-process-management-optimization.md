# Process Management & Startup Scripts Optimization

**Status**: Completed
**Date**: 2026-02-24

## Summary

Systematically reduced duplication between shell scripts (~3,500 lines) and Go layer (~1,800 lines) for process management. Converged toward Go as single source of truth.

## Commits

1. `580768a6` — Phase 1.1: Extract shared component lifecycle library (`scripts/lark/component.sh`)
2. `d364305a` — Phase 1.2: Standardize shutdown timeouts (5s/10s/30s hierarchy)
3. `64f2fbd2` — Phase 1.3: PGID-aware stop_pid() to prevent orphan processes
4. `0585f460` — Phase 2.1: Reduce dev.sh from 825→130 lines, delegate to `alex dev`
5. `90ae55a2` — Phase 2.2: Unified status view with sections and URLs
6. `e10218ce` — Phase 3: Orphan cleanup, PID meta, config dump, build fingerprinting

## Key Changes

### Shell Layer
- **component.sh**: Shared build/start/stop/restart/status/logs functions
- **main.sh**: 219→74 lines (config + delegation)
- **test.sh**: 314→183 lines (config + worktree logic + delegation)
- **kernel.sh**: 168→78 lines (config + delegation)
- **dev.sh**: 825→130 lines (thin dispatcher to `alex dev`)
- **process.sh**: PGID-aware signals, timeout constants, PID meta format

### Go Layer
- **process/manager.go**: Named timeout constants, ScanOrphans/CleanupOrphans
- **services/backend.go**: Correct build target (alex-web), fingerprint-based skip
- **services/web.go**: Orphan cleanup, ENOENT recovery support
- **config.go**: ServerBin default fixed to ./alex-web, AutoHealWebNext added
- **buildinfo/fingerprint.go**: New package for build fingerprinting
- **dev_lark.go**: Caffeinate guard for macOS

## New Commands
- `alex dev cleanup` — Scan and remove orphan PID files
- `alex dev config dump` — Show all resolved configuration
- `alex dev config get <key>` — Get specific config value

## Verification Checklist
- [x] All Go tests pass (`go test ./internal/devops/...`)
- [x] All shell scripts pass syntax check (`bash -n`)
- [x] Go build succeeds (`go build ./...`)
- [x] Go vet clean on changed packages
