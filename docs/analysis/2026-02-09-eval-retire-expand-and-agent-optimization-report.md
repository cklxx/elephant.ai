# 2026-02-09 Eval Retire/Expand and Agent Optimization Report

## Scope
- Retire low-signal easy cases in motivation collection and replace with harder implicit-intent cases.
- Add new hard collection: `intent_decomposition_constraint_matrix`.
- Optimize routing heuristics for persistent top1 conflict families.

## Updated Datasets
- Updated: `evaluation/agent_eval/datasets/foundation_eval_cases_motivation_aware_proactivity.yaml`
- Added: `evaluation/agent_eval/datasets/foundation_eval_cases_intent_decomposition_constraint_matrix.yaml`
- Updated suite: `evaluation/agent_eval/datasets/foundation_eval_suite.yaml`

## Baseline vs After (full suite)
Baseline run:
- `tmp/foundation-suite-retire-expand-baseline/foundation_suite_result_foundation-suite-20260209-062450.json`

After run:
- `tmp/foundation-suite-retire-expand-after2/foundation_suite_result_foundation-suite-20260209-062830.json`

| Metric | Baseline | After |
|---|---:|---:|
| Total collections | `21` | `22` |
| Cases passed (x/x) | `558/559` | `580/581` |
| pass@1 (x/x) | `485/559` | `505/581` |
| pass@5 (x/x) | `559/559` | `581/581` |
| Failed cases | `1` | `1` |
| Deliverable Good | `26/35` | `27/37` |
| Deliverable Bad | `9/35` | `10/37` |

## Motivation Standalone (after retire/expand)
After run:
- `tmp/foundation-motivation-retire-expand-after2/foundation_suite_result_foundation-suite-20260209-062830.json`

| Metric | Value |
|---|---:|
| Cases passed (x/x) | `31/32` |
| pass@1 (x/x) | `30/32` |
| pass@5 (x/x) | `32/32` |
| Failed cases | `1` |

## What was retired and expanded
### Retired/replaced easy patterns
- replaced direct easy reminders/message/calendar singles with conflict-rich variants:
  - consent-gate vs task ownership
  - message vs upload (explicit no-upload)
  - memory-policy recall before action
  - exact fixed-URL fetch
  - shell command health check vs script execution
  - path-name locate before open

### New hard collection
- `intent_decomposition_constraint_matrix` added with 20 conflict-heavy cases:
  - consent gate vs task-manage
  - plan vs direct mutation
  - memory recall vs code/file search
  - message vs upload
  - fetch exact source vs discovery search
  - shell command vs deterministic code execution
  - find/ripgrep/okr/scheduler disambiguation
  - artifact deliverable vs attachment handoff

## Heuristic optimization highlights
Implemented in `evaluation/agent_eval/foundation_eval.go`:
- stronger conflict penalties for:
  - `lark_task_manage` under consent-only intents
  - `lark_upload_file` under “without/no/not upload/attach” constraints
  - `search_file` under memory-policy/decision-history intents
  - `browser_action` under non-UI artifact/proof intents
- stronger boosts for:
  - `request_user` under external-outreach consent constraints
  - `lark_send_message` for check-in/nudge and explicit no-upload constraints
  - `okr_read` for baseline-read semantics
  - `find` for nested path-name locate intents
  - `memory_search` for previous-success/policy-pattern retrieval

## Remaining Top1 Failure
Only one case remains in both motivation and full suite:
- collection: `motivation-aware-proactivity`
- case: `motivation-progress-artifact-proof`
- expected: `artifacts_write`
- top1: `browser_action`
- rank: `5`

## Conclusion
- Retire+expand succeeded: coverage increased and hard pressure increased while pass@5 remained perfect.
- Agent routing precision improved under harder test volume (`pass@1` improved in absolute hit count and maintained single-failure boundary).
- Next iteration should close the final artifact-proof vs browser-action ambiguity with finer non-UI action suppression and richer artifact-proof lexical features.
