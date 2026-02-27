# 2026-02-27 Feature Quality Review and CI Pass

## Goal

- Review every feature from `docs/reference/feature-inventory-code-sourced.md` and produce a consolidated quality report.
- Fix current CI blocker(s) and verify local CI-equivalent checks pass.

## Scope

- In-scope:
  - Web event stream ordering defect currently failing `web` tests.
  - Feature-by-feature quality assessment document in `docs/reviews/`.
  - Full local verification across Go + Web CI gates.
- Out-of-scope:
  - New product features.
  - Large architectural refactors unrelated to current CI blocker.

## Plan

1. [completed] Baseline and quality-gate scan.
2. [completed] Fix deterministic ordering bug in `web/components/agent/eventStreamUtils.ts` and keep tests meaningful.
3. [completed] Re-run web test suite and full CI-equivalent checks.
4. [completed] Write feature-by-feature quality review doc with severity-ranked findings and evidence.
5. [completed] Run mandatory code review skill and address P0/P1 findings.
6. [in_progress] Commit changes.

## Verification Targets

- `npm --prefix web run lint`
- `npm --prefix web run test`
- `npm --prefix web run build`
- `npm --prefix web run analyze`
- `make check-arch`
- `make check-arch-policy`
- `./scripts/run-golangci-lint.sh run ./...`
- `./scripts/go-with-toolchain.sh test -race -covermode=atomic ./...`
