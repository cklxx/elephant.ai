# 2026-02-09 — Harder Case Expansion + Easy pass@1 Retirement

## Context
Foundation suite had many scenarios that repeatedly hit `pass@1` across historical reruns, reducing challenge and masking routing weaknesses.

## What Worked
- Mined historical suite artifacts and identified scenarios with `>=3` runs and always `hit_rank=1`.
- Replaced `16` stable easy cases in core collections with ambiguity/conflict-heavy variants while preserving coverage dimensions.
- Added `12` new hard scenarios to `challenge_hard_v2` with implicit cues and tool-conflict pressure.

## Outcome
- Case volume increased from `493` to `505`.
- Challenge set expanded from `37` to `49` cases.
- New baseline became stricter: `pass@1=383/505`, `pass@5=502/505`, exposing 4 actionable hard failures.

## Reusable Rule
When evaluation score gets too high, retire repeatedly top1-perfect prompts and replace with conflict-driven intents before adding more generic volume.
