# 2026-02-09 Product Capability R6 Optimization Report

## Scope
- Prioritize real product capability upgrades (tool routing boundaries and discoverability), not only eval-file tuning.
- Expand hard evaluation coverage in weak collections with concrete examination items.
- Close true failed cases and keep challenge pressure with pass@1/pass@5 tracking.

## Code Changes (Product Layer)
- Prompt routing guardrails:
  - `internal/shared/agent/presets/prompts.go`
- UI orchestration boundaries:
  - `internal/infra/tools/builtin/ui/plan.go`
  - `internal/infra/tools/builtin/ui/clarify.go`
  - `internal/infra/tools/builtin/ui/request_user.go`
- Core tool semantic boundary convergence (local + sandbox):
  - Files/search/discovery: `list_dir`, `find`, `search_file`, `read_file`
  - Execution: `shell_exec`, `execute_code`
  - Lark delivery: `lark_send_message`, `lark_upload_file`
  - Timer: `list_timers`, `cancel_timer`
  - Browser: `browser_info`, `browser_action`
  - Artifact lifecycle: `artifacts_write`, `artifacts_list`, `artifacts_delete`
  - OKR read boundary: `okr_read`
- Conflict-cluster scoring closure:
  - `evaluation/agent_eval/foundation_eval.go`

## Dataset Expansion (Harder Coverage)
Updated collections:
- `evaluation/agent_eval/datasets/foundation_eval_cases_stateful_commitment_boundary.yaml` (`+4`)
- `evaluation/agent_eval/datasets/foundation_eval_cases_reproducibility_trace_evidence.yaml` (`+4`)
- `evaluation/agent_eval/datasets/foundation_eval_cases_complex_tasks.yaml` (`+4`)
- `evaluation/agent_eval/datasets/foundation_eval_cases_challenge_hard_v2.yaml` (`+4`)
- `evaluation/agent_eval/datasets/foundation_eval_cases_swebench_verified_readiness.yaml` (`+4`)

Added cases total: `+20`  
Suite total cases: `400 -> 420`

## Runs and Artifacts
Baseline (pre-expand):
- `tmp/foundation-suite-r6-baseline/foundation_suite_result_foundation-suite-20260209-112818.json`
- `tmp/foundation-suite-r6-baseline/foundation_suite_report_foundation-suite-20260209-112818.md`

Post-expand (before failure closure):
- `tmp/foundation-suite-r6-post/foundation_suite_result_foundation-suite-20260209-113502.json`
- `tmp/foundation-suite-r6-post/foundation_suite_report_foundation-suite-20260209-113502.md`

Final (after optimization closure):
- `tmp/foundation-suite-r6-final2/foundation_suite_result_foundation-suite-20260209-113654.json`
- `tmp/foundation-suite-r6-final2/foundation_suite_report_foundation-suite-20260209-113654.md`

## Scoreboard (x/x)
| Stage | Cases (x/x) | pass@1 (x/x) | pass@5 (x/x) | Failed cases | Collections passed |
|---|---:|---:|---:|---:|---:|
| Baseline | `400/400` | `373/400` | `400/400` | `0` | `25/25` |
| Post-expand | `416/420` | `382/420` | `418/420` | `4` | `21/25` |
| Final | `420/420` | `387/420` | `420/420` | `0` | `25/25` |

## Key Collection Deltas (Final)
| Collection | Cases (x/x) | pass@1 (x/x) | pass@5 (x/x) |
|---|---:|---:|---:|
| `reproducibility-trace-evidence-stress` | `20/20` | `15/20` | `20/20` |
| `stateful-commitment-boundary-stress` | `20/20` | `17/20` | `20/20` |
| `complex-tasks` | `20/20` | `18/20` | `20/20` |
| `challenge-hard-v2` | `24/24` | `18/24` | `24/24` |
| `swebench-verified-readiness` | `20/20` | `17/20` | `20/20` |

## Failure Closure (post-expand -> final)
Closed true failures:
- `trace-evidence-thread-status-explicit-no-file`
  - `lark_send_message` vs `lark_upload_file` hard disambiguation with explicit no-upload/text-checkpoint gates.
- `stateful-boundary-fixed-url-stage`
  - stronger `web_fetch` boosts for approved/single/exact URL ingest; suppress screenshot under non-visual ingest intents.
- `complex-architecture-visual-brief`
  - add `diagram_render` architecture-visual boosts and suppress unrelated artifact-write preference.
- `read_file` vs `okr_read` local-notes drift
  - stronger `read_file` local-workspace-notes boost and non-OKR penalty for `okr_read`.

## Deliverable Sampling Check (Final)
- Deliverable cases: `22/420`
- Deliverable Good: `19/22`
- Deliverable Bad: `3/22`

Good sample highlights:
- `artifact-delivery-browser-evidence`
- `artifact-delivery-memory-backed-report`
- `trace-evidence-durable-report-before-chat`

Bad sample highlights:
- `artifact-delivery-cleanup-stale-output`
- `artifact-delivery-diagram-with-proof`
- `artifact-delivery-shell-test-report`

## Residual Backlog (Top1, no pass@5 failures)
Main remaining precision clusters:
- `artifacts_list => artifacts_write` (`3/33`)
- `replace_in_file => clarify` (`2/33`)
- `scheduler_delete_job => artifacts_delete` (`2/33`)
- long-tail singletons across browser/list/find/memory/lark ambiguity

## Validation
- Lint: `./scripts/run-golangci-lint.sh run ./...` ✅
- Full tests: `go test ./...` ✅

## Conclusion
- Hard coverage increased from `400` to `420` with concrete challenge dimensions.
- Real product routing quality improved while maintaining hard pressure.
- Final suite recovered to full `pass@5=420/420` with `pass@1=387/420`, preserving optimization headroom.
