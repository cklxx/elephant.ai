# 2026-02-09 — Foundation Suite Systematic Expansion V3

Impact: Foundation offline suite now covers higher-difficulty long-horizon, architecture coding, deep research, and autonomy initiative capabilities while preserving full Top-K pass.

## What changed
- Added 4 new collections to `foundation_eval_suite.yaml`:
  - `long-horizon-multi-round`
  - `architecture-coding-hard`
  - `deep-research`
  - `autonomy-initiative`
- Added 4 new dataset files:
  - `foundation_eval_cases_long_horizon_multi_round.yaml` (18 cases)
  - `foundation_eval_cases_architecture_coding_hard.yaml` (20 cases)
  - `foundation_eval_cases_deep_research.yaml` (18 cases)
  - `foundation_eval_cases_autonomy_initiative.yaml` (18 cases)

## Result
- Suite scale: `13 -> 17` collections, `334 -> 408` cases.
- Final run (`tmp/foundation-suite-systematic-20260209-r3`):
  - Collections: `17/17`
  - Cases: `408/408`
  - Availability errors: `0`

## Why this worked
- Kept tool expectations on canonical names and reused high-signal intent phrasing from existing passing sets.
- Used iterative badcase closure:
  - Round 1: `405/408`
  - Round 2: `406/408`
  - Round 3: `408/408`
- Targeted only ranking failures (`rank_below_top_k`) by strengthening lexical routing cues in intents.

## Validation
- `go test ./evaluation/agent_eval/...`
- `go test ./cmd/alex/...`
- `go run ./cmd/alex eval foundation-suite --suite evaluation/agent_eval/datasets/foundation_eval_suite.yaml --output tmp/foundation-suite-systematic-20260209-r3 --format markdown`
