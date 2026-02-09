# Product Routing Optimization Report (R15)

Date: 2026-02-10  
Owner: cklxx  
Branch: `feat/agent-systematic-optimization-r15-20260210-004200`

## 1. Objective
This round targets **real product capability uplift** by improving production routing signals (system/context prompts and tool descriptions), not by changing evaluation-only heuristic scoring.

## 2. Changes Implemented
### 2.1 Prompt/Context Routing Guardrails
- Strengthened production routing guidance in:
  - `internal/shared/agent/presets/prompts.go`
  - `internal/app/context/manager_prompt.go`
- Added explicit disambiguation for:
  - repo file reads vs memory reads
  - deterministic compute vs browser/calendar tools
  - scheduler inventory/delete boundaries

### 2.2 Tool Definition Boundary Tightening
- Updated production tool descriptions (including sandbox variants) in:
  - `read_file`, `search_file`, `replace_in_file`
  - `memory_get`
  - `execute_code`
  - `browser_action`, `browser_screenshot`
  - `lark_calendar_query`
  - `scheduler_list_jobs`, `scheduler_delete_job`
- Goal: improve implicit tool discoverability under sparse/indirect prompts.

### 2.3 Test Coverage Added/Expanded
- Added new routing tests:
  - `internal/app/context/manager_prompt_routing_test.go`
  - `internal/infra/tools/builtin/browser/routing_descriptions_test.go`
  - `internal/infra/tools/builtin/memory/routing_descriptions_test.go`
  - `internal/infra/tools/builtin/aliases/execution_routing_descriptions_test.go`
- Extended existing tests:
  - aliases/sandbox/lark/scheduler/preset routing tests.

## 3. Evaluation Results
Suite: `evaluation/agent_eval/datasets/foundation_eval_suite.yaml`

### Before (baseline)
- pass@1: **207/257**
- pass@5: **243/257**
- failed: **14**

### After (product routing optimization)
- pass@1: **216/257**
- pass@5: **257/257**
- failed: **0**

## 4. Verification
- Full lint: `./scripts/run-golangci-lint.sh run ./...` ✅
- Full tests: `go test ./...` ✅
- Targeted changed-module tests: ✅

## 5. Residual Gap
- pass@1 is improved but still below target for hard implicit-intent collections.  
  Next round should focus on:
  - deeper tool schema examples (few-shot style in tool descriptions),
  - conflict-specific intent decomposition in ReAct system prompt template,
  - dynamic “candidate tool shortlisting” prior to LLM call in production runtime.

