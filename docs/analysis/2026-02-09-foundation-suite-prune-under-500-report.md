# 2026-02-09 — Foundation Suite Prune Under 500 Report

## 1) Objective
- Reduce foundation suite size to below `500` cases.
- Do pruning (retirement) instead of only expansion.
- Keep full pass@5 coverage and preserve high-conflict dimensions.

## 2) Scoreboard (x/x)

| Stage | Collections | Cases | pass@1 | pass@5 | Failed | Deliverable Good/All |
|---|---:|---:|---:|---:|---:|---:|
| Before prune | `25/25` | `656/656` | `562/656` | `656/656` | `0` | `39/50` |
| After prune | `25/25` | `499/499` | `426/499` | `499/499` | `0` | `26/32` |

Case volume change:
- Added: `0`
- Retired: `157`
- Net: `-157`

## 3) Pruning Policy
- Enforced per-collection hard caps to stop uncontrolled growth.
- Retained all 25 dimensions, but reduced low-signal/duplicate volume inside each collection.
- Kept hard stress collections (`sparse_clue/stateful/reproducibility`) intact at `16` each.

## 4) New Collection Sizes (after prune)
- tool-coverage: `30`
- prompt-effectiveness: `20`
- proactivity: `24`
- motivation-aware-proactivity: `20`
- complex-tasks: `24`
- availability-recovery: `20`
- valuable-workflows: `22`
- swebench-verified-readiness: `20`
- multi-step-orchestration: `17`
- safety-boundary-policy: `17`
- context-learning-hard: `17`
- memory-capabilities: `22`
- user-habit-soul-memory: `22`
- task-completion-speed: `15`
- long-horizon-multi-round: `16`
- architecture-coding-hard: `17`
- deep-research: `16`
- autonomy-initiative: `16`
- conflict-convergence-hard: `20`
- intent-decomposition-constraint-matrix: `20`
- challenge-hard-v2: `32`
- complex-artifact-delivery: `24`
- sparse-clue-retrieval-stress: `16`
- stateful-commitment-boundary-stress: `16`
- reproducibility-trace-evidence-stress: `16`

## 5) Remaining Top1 Conflict Backlog (after prune)
- `web_fetch => web_search` (`3`)
- `plan => lark_task_manage` (`3`)
- `find => search_file` (`3`)
- `ripgrep => search_file` (`2`)
- `request_user => clarify` (`2`)
- `memory_search => write_file/search_file/clarify` (`6` aggregated)

## 6) Deliverable Sampling
- Deliverable cases: `32`
- Good: `26`
- Bad: `6`

Interpretation:
- Suite is now controllable (`<500`) and still diagnostic.
- pass@5 remains complete, so routing coverage is preserved while reducing maintenance cost.

## 7) Run Artifacts
- baseline: `tmp/foundation-suite-prune-baseline-20260209-175115`
- final: `tmp/foundation-suite-prune-final-20260209-175157`
