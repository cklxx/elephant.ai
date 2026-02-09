# 2026-02-09 Rename Intent Decomposition Eval Set

## Goal
Replace vague collection naming with a systematic, concrete examination-oriented name and taxonomy.

## Changes
- Rename dataset file:
  - from `foundation_eval_cases_agent_optimization_hard_v3.yaml`
  - to `foundation_eval_cases_intent_decomposition_constraint_matrix.yaml`
- Rename collection metadata in suite:
  - id: `intent-decomposition-constraint-matrix`
  - dimension: `intent_decomposition_constraint_matrix`
  - name: `Intent Decomposition Constraint Matrix`
- Refine case categories into explicit examination items (consent gating, planning boundary, memory retrieval, channel action selection, retrieval mode, execution mode, scheduling semantics, deliverable routing).
- Sync README/report/experience docs to new names.

## Verification
- Run `go test ./evaluation/agent_eval/...`
- Run foundation suite with updated path and confirm successful load.
