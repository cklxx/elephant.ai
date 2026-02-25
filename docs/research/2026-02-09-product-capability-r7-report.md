# 2026-02-09 Product Capability Optimization (R7) Report

## Scope
- Objective: improve real product tool-routing quality under implicit prompts and conflict-heavy intents.
- Method: optimize product-layer routing signals (system prompt + tool definition semantics), then validate via full foundation suite.

## Runs
- Baseline: `tmp/foundation-suite-r7-baseline/foundation_suite_report_foundation-suite-20260209-114434.md`
- Iteration-1 (regressed, discarded): `tmp/foundation-suite-r7-after-routing/foundation_suite_report_foundation-suite-20260209-115032.md`
- Final (adopted): `tmp/foundation-suite-r7-after-routing-v2/foundation_suite_report_foundation-suite-20260209-115214.md`

## Headline Results (x/x)
| Metric | Baseline | Final | Delta |
|---|---:|---:|---:|
| pass@1 | 387/420 | 389/420 | +2 |
| pass@5 | 420/420 | 420/420 | 0 |
| Top1 misses | 33/420 | 31/420 | -2 |
| Failed cases | 0/420 | 0/420 | 0 |
| Deliverable good | 19/22 | 19/22 | 0 |
| Deliverable bad | 3/22 | 3/22 | 0 |

## What Changed (Product Code)
- Strengthened global routing guardrails for implicit intent decomposition:
  - `internal/shared/agent/presets/prompts.go`
  - `internal/app/agent/preparation/service.go`
- Added explicit tool-boundary semantics across conflict-prone tools:
  - Lark context vs message vs upload:
    - `internal/infra/tools/builtin/larktools/chat_history.go`
    - `internal/infra/tools/builtin/larktools/send_message.go`
    - `internal/infra/tools/builtin/larktools/upload_file.go`
  - Browser read-only vs interaction:
    - `internal/infra/tools/builtin/browser/info.go`
    - `internal/infra/tools/builtin/browser/actions.go`
    - `internal/infra/tools/builtin/sandbox/sandbox_browser.go`
  - Artifact lifecycle boundaries:
    - `internal/infra/tools/builtin/artifacts/artifacts.go`
  - File search/replace boundaries:
    - `internal/infra/tools/builtin/aliases/search_file.go`
    - `internal/infra/tools/builtin/aliases/replace_in_file.go`
    - `internal/infra/tools/builtin/sandbox/sandbox_file.go`
  - Scheduler delete scope:
    - `internal/infra/tools/builtin/scheduler/scheduler_delete.go`
  - Clarify overuse suppression:
    - `internal/infra/tools/builtin/ui/clarify.go`

## Regression Guard Tests Added
- `internal/app/agent/preparation/default_prompt_test.go`
- `internal/infra/tools/builtin/aliases/routing_descriptions_test.go`
- `internal/infra/tools/builtin/larktools/routing_descriptions_test.go`
- `internal/infra/tools/builtin/sandbox/routing_descriptions_test.go`
- Updated:
  - `internal/shared/agent/presets/presets_test.go`
  - `internal/infra/tools/builtin/artifacts/artifacts_test.go`
  - `internal/infra/tools/builtin/scheduler/scheduler_tools_test.go`

## Top Remaining Badcase Clusters (Final)
| Cluster | Misses (x/x) | Notes |
|---|---:|---|
| `artifacts_list => artifacts_write` | 3/31 | inventory intents still occasionally map to creation |
| `memory_search => lark_chat_history` | 2/31 | memory preference recall vs chat-history context still overlaps |
| `scheduler_delete_job => artifacts_delete` | 2/31 | delete semantics conflict across scheduler/artifact cleanup |

## Sample Good Case (Final)
- Case: `artifact-delivery-search-proof`
- Expected: `search_file, artifacts_write`
- Top matches: `artifacts_write`, `search_file`, `artifacts_list`
- Coverage: contract `3/3`, 100%

## Sample Bad Case (Final)
- Case: `proactive-send-progress-update`
- Expected: `lark_send_message`
- Top1: `lark_upload_file`
- Failure mode: "send/update/chat" lexical overlap still occasionally overweights upload path despite no-file intent.

## Validation
- Lint: `./scripts/run-golangci-lint.sh run ./...` passed.
- Full tests: `go test ./... -count=1` passed.
- Note: sqlite cgo deprecation warnings observed on macOS SDK (non-blocking).

