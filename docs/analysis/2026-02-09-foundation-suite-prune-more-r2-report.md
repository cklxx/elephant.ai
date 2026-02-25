# 2026-02-09 — Foundation Suite Further Prune Review (R2)

## 1) Objective
- Further reduce case volume after first prune (`499`) because additional redundancy remained.
- Preserve all 25 dimensions and maintain full pass@5.

## 2) x/x Scoreboard

| Stage | Collections | Cases | pass@1 | pass@5 | Failed | Deliverable Good/All |
|---|---:|---:|---:|---:|---:|---:|
| Previous prune baseline | `25/25` | `499/499` | `426/499` | `499/499` | `0` | `26/32` |
| R2 further prune | `25/25` | `445/445` | `375/445` | `445/445` | `0` | `23/28` |

Volume delta:
- Added: `0`
- Retired: `54`
- Net: `-54`

## 3) Pruning Strategy
- Kept all hard stress collections at `16` each.
- Reduced large general collections with lower marginal information gain:
  - `tool_coverage 30 -> 24`
  - `challenge_hard_v2 32 -> 24`
  - `complex_artifact_delivery 24 -> 20`
  - `complex_tasks 24 -> 20`
  - `proactivity 24 -> 20`
  - `memory/user_habit/workflows 22 -> 18`
- Reduced medium collections to `16~18` caps while keeping dimension presence.

## 4) Current Suite Size (after R2)
- Total collections: `25`
- Total cases: `445`
- Target (<500): achieved

## 5) Residual Top1 Conflict Backlog
- `web_fetch => web_search` (`3`)
- `plan => lark_task_manage` (`3`)
- `find => search_file` (`3`)
- `memory_search => write_file/search_file/clarify` (`6` aggregated)
- `lark_send_message => replace_in_file/lark_upload_file` (`4` aggregated)

## 6) Interpretation
- Suite is significantly leaner and still fully pass@5 covered.
- Diagnostic conflict pressure remains (top1 misses still clustered), so optimization signal is preserved.
- Maintenance cost decreases with retained benchmark-style hard dimensions.

## 7) Artifacts
- R2 final run: `tmp/foundation-suite-prune-r2-final-20260209-182001`
