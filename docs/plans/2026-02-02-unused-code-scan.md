# Plan: Unused Code Scan (2026-02-02)

## Goal
Identify code in this repo that appears unused (not referenced or reachable) and report candidates with evidence.

## Scope
- Go backend packages under `cmd/`, `internal/`, `tests/`.
- Web/TS under `web/` (unused exports/vars).
- Exclude generated artifacts and `node_modules/`.

## Plan
- [x] Inventory build/lint tooling and decide scanners per language.
- [x] Run unused-code scanners (Go + Web) and collect findings.
- [x] Triage results (confirm references, note false positives).
- [x] Report candidates with file paths and rationale.

## Updates
- 2026-02-02 16:00: Plan created.
- 2026-02-02 16:05: Inventory + scanners run; triaged unused packages and unparam warnings.
- 2026-02-02 16:07: Report prepared; lint/tests + dev restart completed.
