# sqlite-vec-go-bindings (local fork)

This directory is a local fork of:

- Module: `github.com/asg017/sqlite-vec-go-bindings`
- Base version: `v0.1.6`
- Upstream repo: <https://github.com/asg017/sqlite-vec-go-bindings>

## Why this fork exists

macOS deprecates process-global SQLite auto-extension APIs (`sqlite3_auto_extension` and `sqlite3_cancel_auto_extension`), which emitted warnings during CGO builds.

To remove that warning path while keeping sqlite-vec support:

- Added `InitDBHandle(dbHandle uintptr) error` for connection-scoped initialization.
- Split auto-extension behavior by platform:
  - `cgo/auto_non_darwin.go`: preserves upstream `Auto`/`Cancel`.
  - `cgo/auto_darwin.go`: `Auto`/`Cancel` are no-ops by design.

In this repo, sqlite-vec is initialized via a `github.com/mattn/go-sqlite3` `ConnectHook` on each connection.

## Update process

1. Sync upstream `v0.1.x` cgo sources (`cgo/sqlite-vec.c`, `cgo/sqlite-vec.h`, and related Go files).
2. Re-apply local patch files:
   - `cgo/lib.go`
   - `cgo/auto_non_darwin.go`
   - `cgo/auto_darwin.go`
3. Run full repo lint/tests and verify Darwin build is warning-free.
