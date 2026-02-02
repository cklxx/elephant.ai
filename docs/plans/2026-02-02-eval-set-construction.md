# Plan: Evaluation set construction (baseline + challenge)

Owner: cklxx
Date: 2026-02-02

## Goal
Design and implement evaluation set structure, rubric, and automation flow for baseline gate + challenge evaluations, including agent-judged criteria and automatic scoring rules.

## Steps
- [ ] Inspect existing evaluation suites (`evaluation/`, scripts, CI) and summarize current capabilities/gaps.
- [ ] Draft evaluation set taxonomy, dataset layout, and scoring criteria (baseline gate + challenge).
- [ ] Define automation flow: data ingestion → execution → auto-judging → agent-judging → report/baseline comparison.
- [ ] Implement/update docs + config/templates needed for the evaluation set.
- [ ] Add tests or validation hooks for the new evaluation pipeline; run full lint/tests.
- [ ] Restart dev services; commit in incremental steps.
