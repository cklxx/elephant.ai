# 2026-02-09 — pass@1 Optimization Round 3

## Context
Round2 reached `390/505`; remaining top1 misses clustered into a few conflict pairs.

## What Worked
- Used pairwise conflict analysis from case-level miss export instead of broad retuning.
- Added disambiguation for:
  - `request_user` vs `clarify`
  - `lark_upload_file` vs `write_attachment`
  - `memory_search` vs `search_file`
  - `cancel_timer` vs `set_timer`
- Added regression tests that assert these new directional preferences.

## Outcome
- pass@1 improved to `398/505`.
- pass@5 stayed `505/505`.
- No new failed cases introduced.

## Reusable Rule
When pass@5 is stable, prioritize top1 miss pair-frequency analysis and tune only the dominant conflict pairs.
