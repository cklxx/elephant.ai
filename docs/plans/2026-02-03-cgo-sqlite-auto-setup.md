# Plan: CGO SQLite Setup + Auto Mode

Owner: cklxx
Date: 2026-02-03

## Goal
Provide a repeatable CGO dependency setup for sqlite-vec and auto-select CGO for local builds when possible, while keeping tests on macOS defaulting to CGO-disabled unless explicitly enabled.

## Scope
- Add a CGO/sqlite dependency install script for macOS + Linux.
- Add shared CGO detection helpers.
- Update dev tooling to auto-select CGO for builds (with explicit override).
- Keep memory search logic unchanged; no backend changes.

## Non-Goals
- Replace sqlite-vec with Postgres or other vector backends.
- Silence sqlite-vec deprecation warnings beyond optional environment flags.
- Change production deployment workflows.

## Plan of Work
1) Add CGO detection helpers in scripts/lib/common.
2) Add `scripts/setup_cgo_sqlite.sh` and wire a `dev.sh setup-cgo` command.
3) Update dev build flow to auto-enable CGO when available (respecting overrides).
4) Update docs where needed and refresh long-term memory timestamp.
5) Run `./dev.sh lint` and `./dev.sh test`.

## Test Plan
- `./dev.sh test`
- `./dev.sh lint`

## Progress
- [x] CGO detection helpers
- [x] Setup script + dev.sh command
- [x] Auto-CGO build selection
- [x] Docs + memory timestamp
- [x] Tests (go test failed in lark/testing loop-selfheal-smoke; eslint missing for web lint)
