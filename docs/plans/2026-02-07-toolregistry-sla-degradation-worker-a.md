# 2026-02-07 Toolregistry SLA-Aware Degradation (worker-A)

## Background
- Tool degradation exists but does not use SLA router health signals.
- Fallback execution currently keeps the original `call.Name`, which misattributes downstream behavior/metrics.
- Registry wrapping path does not selectively inject degradation based on configured fallback mappings.

## Goal
- Add SLA-aware fallback candidate ordering with optional pre-route when primary is unhealthy.
- Ensure fallback execution rewrites `call.Name` to the fallback tool name.
- Wire degradation wrapping in registry only for mapped tools, while preserving existing wrap compatibility.

## Plan
1. Extend degradation config/executor for SLA-aware ordering + optional pre-route.
2. Update fallback execution call rewriting and keep metadata annotations.
3. Add registry-level degradation defaults and selective wrapping logic.
4. Keep `wrapTool(...)` compatibility and extend unwrap logic for new layer.
5. Add/adjust tests and run `gofmt` + `go test ./internal/app/toolregistry`.

## Progress
- [x] Reviewed current `degradation` and `registry` implementation and tests.
- [x] Implemented degradation executor SLA-routing + fallback call-name rewrite.
- [x] Implemented registry selective degradation wrapping with safe defaults.
- [x] Updated tests for degradation and registry behavior.
- [x] Ran formatting and package tests.
- [x] Ran repo test suite; lint currently fails due unrelated pre-existing issue.

## Acceptance
- Degradation chain supports SLA-ordered fallbacks and optional pre-route when primary is unhealthy.
- Fallback invocation uses rewritten `call.Name` to the selected fallback.
- Registry only wraps tools with degradation when they exist in fallback map.
- Existing `wrapTool` callers/tests remain compatible.
