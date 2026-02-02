# Plan: Evaluation set construction (baseline + challenge)

Owner: cklxx
Date: 2026-02-02

## Goal
Design and implement evaluation set structure, rubric, and automation flow for baseline gate + challenge evaluations, including agent-judged criteria and automatic scoring rules.

## Steps
- [x] Inspect existing evaluation suites (`evaluation/`, scripts, CI) and summarize current capabilities/gaps.
- [x] Draft evaluation set taxonomy, dataset layout, and scoring criteria (baseline gate + challenge).
- [x] Define automation flow: data ingestion → execution → auto-judging → agent-judging → report/baseline comparison.
- [x] Implement/update docs + config/templates needed for the evaluation set.
- [ ] Add tests or validation hooks for the new evaluation pipeline; run full lint/tests.
- [x] Restart dev services; commit in incremental steps.

## Notes
- `./dev.sh lint` currently fails in unrelated files (`internal/memory/decision_test.go`, `internal/memory/preferences_test.go`, `internal/skills/custom.go`, `evaluation/gate/gate.go`).
- `./dev.sh test` stalled after ~10 minutes running `go test -race ./...`; terminated to unblock workflow.
- `./dev.sh down && ./dev.sh` completed; dev services restarted successfully.
