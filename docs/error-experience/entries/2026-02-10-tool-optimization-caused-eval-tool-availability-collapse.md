# Tool Optimization Caused Eval Tool Availability Collapse

**Date:** 2026-02-10
**Severity:** P1 — blocks meaningful capability evaluation
**Component:** Tool registration / preset-toolset availability in evaluation mode

## Symptom

After tool large-scale optimization, foundation evaluations showed massive `N/A` and `availability_error`:
- E2E suite: `N/A 177`, `failed 5`, `deliverable good 0/20`
- Current suite: `N/A 138`, `failed 2`, `deliverable good 0/30`

## Root Cause

In `web/full/default` evaluation context, only 14 tools remained discoverable.  
Many benchmark-critical tools became unavailable (`find`, `list_dir`, `search_file`, `ripgrep`, `artifacts_*`, `artifact_manifest`, `lark_*`, `write_attachment`, etc.), causing evaluation to measure availability gaps instead of routing intelligence.

## Impact

- pass@1/pass@5 degraded materially versus same-day baseline.
- Deliverable checks collapsed to `0` good because artifact/delivery tools were unavailable.
- Evaluation conclusions became unreliable for “product capability uplift”.

## Remediation

1. Restore core tool registration coverage for `web/full/default`.
2. Add pre-eval tool inventory gate (expected tools set diff).
3. Treat high `N/A` spikes as release-blocking regression before semantic tuning.

## Lesson

For agent evaluations, tool availability is a hard prerequisite.  
Any major tool refactor must ship with inventory parity checks, otherwise score deltas are dominated by missing tools, not model/strategy quality.
