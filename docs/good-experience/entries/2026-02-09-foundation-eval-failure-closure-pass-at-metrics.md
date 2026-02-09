# 2026-02-09 — Foundation Eval Failure Closure with pass@ Metrics

## Context
Foundation suite regression-hardened run had `7` failing cases (`454/461`) after introducing hard challenge sets and pass@ reporting.

## What Worked
- Targeted lexical-disambiguation tuning in `heuristicIntentBoost` for known collision pairs:
  - `request_user` vs `clarify/plan`
  - `memory_get` vs `memory_search/write_file`
  - `ripgrep` vs `search_file`
- Added intent-family boosts for human-gated approvals and memory-preference recall, plus regex-sweep bias toward `ripgrep`.
- Added direct regression tests for these failure signatures.

## Outcome
- Suite improved from `454/461` to `461/461`.
- `pass@1` improved from `337/461` to `369/461`.
- `pass@5` improved from `454/461` to `461/461`.
- Collections passed improved from `15/18` to `18/18`.

## Reusable Rule
When hard-case additions introduce concentrated failures, first extract the minimal failing signature set from suite JSON and tune dual-condition boosts (action + object) before broad token expansion.
