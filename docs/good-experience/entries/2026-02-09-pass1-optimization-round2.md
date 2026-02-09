# 2026-02-09 — pass@1 Optimization Round 2 on Harder Suite

## Context
After easy-case retirement and hard-case expansion, foundation suite dropped to `pass@1=383/505` with four targeted hard failures.

## What Worked
- Used failure-signature-specific boosts/penalties instead of broad token inflation.
- Fixed four collision families:
  - durable artifact delivery (`artifacts_write` vs `write_attachment`)
  - path inventory (`list_dir` vs `read_file`)
  - memory retrieval stage (`memory_search` vs `memory_get`)
  - source-discovery stage (`web_search` vs `web_fetch` / browser tools)
- Added direct heuristic regression checks.

## Outcome
- pass@1 improved to `390/505`.
- pass@5 restored to `505/505`.
- failed cases reduced `4 -> 0`.

## Reusable Rule
For hard suites, optimize with conflict-pair discriminators and stage-aware penalties instead of adding generic synonym boosts.
