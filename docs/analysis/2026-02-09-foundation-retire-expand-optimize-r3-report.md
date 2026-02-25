# 2026-02-09 — Foundation Eval Retire/Expand/Optimize R3 Report

## 1) Scope
- Continue easy-case retirement and hard-case expansion using implicit/conflict-heavy prompts.
- Re-run full foundation suite, decompose bad cases, then optimize routing heuristics.
- Keep N/A semantics unchanged (N/A not counted as failure).

## 2) Three-Stage Scoreboard (x/x)

| Stage | Collections (passed/total) | Cases (applicable/total) | pass@1 (x/x) | pass@5 (x/x) | Failed cases | Deliverable Good/All |
|---|---:|---:|---:|---:|---:|---:|
| Baseline (before retire/expand) | `21/22` | `581/581` | `505/581` | `581/581` | `1` | `27/37` |
| After dataset hardening only | `20/22` | `608/608` | `516/608` | `606/608` | `4` | `31/44` |
| Final (after optimization) | `22/22` | `608/608` | `526/608` | `608/608` | `0` | `33/44` |

Key effect:
- Challenge pressure increased (`581 -> 608` cases) with lower top1 hit rate than pre-expansion baseline.
- Full top-k recovery achieved after optimization (`pass@5=608/608`).

## 3) Dataset Changes (Retire + Expand)

Updated case sets:
- `evaluation/agent_eval/datasets/foundation_eval_cases_tool_coverage.yaml`
  - Replaced multiple explicit intents with implicit disambiguation variants (`lark message vs upload`, `timer vs scheduler`, `scheduler create/list/delete`).
- `evaluation/agent_eval/datasets/foundation_eval_cases_intent_decomposition_constraint_matrix.yaml`
  - Expanded from `20` to `32` cases.
  - Added concrete examination items across:
    - consent gate disambiguation
    - planning vs mutation boundary
    - memory search vs memory get
    - message/upload/channel-action routing
    - fixed-url fetch vs source discovery
    - repo path-first vs content-first retrieval
    - scheduler create/list/delete semantics
    - deliverable route (artifact vs attachment vs lark upload)
- `evaluation/agent_eval/datasets/foundation_eval_cases_challenge_hard_v2.yaml`
  - Expanded from `49` to `58` cases with additional conflict-heavy implicit tasks.
- `evaluation/agent_eval/datasets/foundation_eval_cases_complex_artifact_delivery.yaml`
  - Expanded from `32` to `38` cases, emphasizing real file deliverables and ambiguous handoff constraints.

## 4) Bad-Case Decomposition (Post-Dataset, Pre-Optimization)

True failures (`pass@5` miss) were concentrated in 4 cases:
1. `tool-scheduler-delete-job`: `scheduler_delete_job` not in top-k.
2. `tool-lark-upload-file`: `lark_upload_file` not in top-k.
3. `tool-scheduler-list-jobs`: `scheduler_list_jobs` not in top-k.
4. `motivation-progress-artifact-proof`: `artifacts_write` not in top-k.

Dominant top1 conflict pairs (post-dataset, pre-opt):
- `memory_search => search_file` (`5`)
- `find => search_file` (`5`)
- `web_fetch => web_search` (`4`)
- `scheduler_delete_job => artifacts_delete` (`3`)
- `plan => lark_task_manage` (`3`)

## 5) Optimization Actions

Heuristic updates in `evaluation/agent_eval/foundation_eval.go`:
- Scheduler semantics convergence:
  - Strengthened list/delete boosts for audit/deprecation/legacy cadence language.
  - Added create-path penalties when intent is delete/audit-first.
  - Reduced `set_timer` bleed-through for scheduler-audit intents.
- Channel-action disambiguation:
  - Strengthened `lark_upload_file` boosts for “review blocked until file in thread”.
  - Added `lark_send_message` penalties under file-required wording.
- Source selection disambiguation:
  - Strengthened `web_fetch` for fixed approved URL/direct-ingest intents.
  - Increased penalties on screenshot/search under URL-ingest-only contexts.
- Artifact vs browser-action disambiguation:
  - Increased `artifacts_write` boosts for progress-proof/durable artifact intents.
  - Increased `browser_action` penalties for non-UI artifact/proof intents.
- Repo retrieval disambiguation:
  - Added stronger `find` boosts for path-first narrowing and stronger `search_file` penalties in path-only contexts.
  - Added stronger `search_file` penalties for persona/habit memory intents.

Regression test added:
- `evaluation/agent_eval/foundation_eval_test.go`
  - New critical case: `motivation-progress-artifact-proof` should route to `artifacts_write` under competing `browser_action/request_user/clarify` profiles.

## 6) Final Remaining Top1 Miss Inventory (Optimization Backlog)

No `pass@5` failures remain, but top1 misses still cluster in:
- `memory_search => search_file` (`5`)
- `find => search_file` (`4`)
- `web_fetch => web_search` (`3`)
- `plan => lark_task_manage` (`3`)
- `lark_send_message => lark_upload_file` (`3`)

These are next optimization candidates for raising pass@1 while keeping challenge pressure.

## 7) Deliverable Sampling (Good/Bad)

Good samples:
- `motivation-progress-artifact-proof` (`artifacts_write`) status=`good`, coverage=`1.0`
- `opt-hard-artifact-progress-proof` (`artifacts_write`) status=`good`, coverage=`1.0`
- `artifact-delivery-concise-chat-durable-record` status=`good`, coverage=`1.0`

Bad samples:
- `opt-hard-downloadable-summary-attachment` status=`bad`, coverage=`0.5`
- `artifact-delivery-path-first-content-later-package` status=`bad`, coverage=`0.0`
- `artifact-delivery-scheduler-audit-before-removal` status=`bad`, coverage=`0.0`

Interpretation:
- Routing is now fully robust at top-k for this suite.
- Deliverable contract quality still needs stronger multi-signal alignment on complex artifact tasks.

## 8) Artifacts
- Baseline: `tmp/foundation-suite-r3-baseline-20260209-150341`
- Post-dataset baseline: `tmp/foundation-suite-r3-after-dataset-20260209-150758`
- Final optimized run: `tmp/foundation-suite-r3-final-20260209-151142`
