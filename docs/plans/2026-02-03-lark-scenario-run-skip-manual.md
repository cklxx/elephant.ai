# Lark scenario runner skips manual scenarios by default

**Date**: 2026-02-03
**Status**: Completed
**Author**: cklxx

## Context
The FAST gate runs `go run ./cmd/alex lark scenario run --dir tests/scenarios/lark ...` before `go test ./...`.
The YAML suite includes `loop-selfheal-smoke` tagged `manual` with an intentionally wrong assertion, so running it by default causes the gate to fail.

## Goal
Make `alex lark scenario run` skip `manual` scenarios by default (consistent with `go test`), while still allowing manual scenarios to be run explicitly.

## Plan
1. Filter out `manual` scenarios unless explicitly selected (name or `--tag manual`).
2. Add unit tests covering default skip + explicit run behavior.
3. Run `go test ./...`, `./dev.sh lint`, and `./dev.sh test`.

## Progress Log
- 2026-02-03: Plan created.
- 2026-02-03: Skipped `manual` scenarios by default in `alex lark scenario run` and added unit tests for default skip + explicit run.
- 2026-02-03: Ran `./dev.sh lint` and `./dev.sh test` (pass).
