# 2026-03-04 Reuse Catalog and Folder Governance

Status: completed
Owner: codex
Updated: 2026-03-04

## Goal

Implement file-level governance rules that make file placement and reuse decisions deterministic and enforce anti-duplication review gates.

## Scope

1. Define reusable capability catalog and mark no-reimplementation boundaries.
2. Define file-type-to-directory ownership boundaries and naming rules.
3. Define component placement, test layering, and task/status file conventions.
4. Define review gate and precheck workflow for anti-duplication.

## Deliverables

- [x] `docs/reference/reuse-catalog-and-folder-governance.md`
- [x] `docs/reference/reuse-path-index.md`
- [x] `docs/guides/reuse-precheck.md`
- [x] `scripts/check-reuse-precheck.sh`
- [x] Update `docs/reference/README.md`
- [x] Update `docs/guides/README.md`
- [x] Update `docs/plans/README.md`

## Progress Log

- 2026-03-04: Verified current orchestration/process/config paths and existing sidecar behavior (`run_tasks`, `taskfile`, process manager, env loader).
- 2026-03-04: Added canonical governance doc with strict file-type mapping, naming policy, and blocking review conditions.
- 2026-03-04: Refined `internal/**` placement to first-level and responsibility-level routing matrix to remove ambiguity.
- 2026-03-04: Added path index and precheck guide with mandatory commands and replacement-reason template.
- 2026-03-04: Added advisory precheck script for changed files.
- 2026-03-04: Updated indexes and engineering practice rule to enforce file-granularity in governance outputs.

## Validation

- `bash scripts/check-reuse-precheck.sh`
- `bash -n scripts/check-reuse-precheck.sh`
