# Evaluation Set Construction

This document defines the baseline/challenge evaluation sets, judging rubric, and the auto/agent judgement workflow.

## Layered Evaluation Sets

**Baseline (准出评测)**  
- Purpose: gate for core reliability on medium-difficulty tasks.  
- Source: `evaluation/agent_eval/datasets/general_agent_eval.json`  
- Definition: `configs/eval/sets/baseline.yaml`

**Challenge (挑战性评测)**  
- Purpose: stress high‑risk tasks to guide module upgrades.  
- Source: `evaluation/agent_eval/datasets/general_agent_eval.json`  
- Definition: `configs/eval/sets/challenge.yaml`

## Eval Set Definition (YAML)

```yaml
version: "1.0.0"
name: "baseline"
description: "Baseline gate for general agent tasks (medium difficulty)."
mode: "baseline"
dataset:
  type: general_agent
  path: evaluation/agent_eval/datasets/general_agent_eval.json
  limit: 0
filters:
  difficulty: ["medium"]
composition_rules:
  - difficulty: "medium"
    min_count: 1
rubric_path: configs/eval/rubrics/baseline.yaml
```

To run an eval set via the evaluation manager/CLI, set:
- `DatasetType = "eval_set"`
- `DatasetPath = configs/eval/sets/<baseline|challenge>.yaml`

## Judging Rubrics

Rubrics live in YAML under `configs/eval/rubrics/`.  
Baseline emphasizes completion, format, and constraint coverage; challenge increases correctness + tradeoff weighting.

```yaml
version: "1.0"
name: "baseline"
pass_threshold: 0.75
fail_on_zero:
  - completion
  - format
  - constraints
dimensions:
  - id: completion
    name: Completion
    weight: 0.15
    auto: true
  - id: format
    name: Format adherence
    weight: 0.20
    auto: true
  - id: constraints
    name: Constraint coverage
    weight: 0.25
    auto: true
  - id: correctness
    name: Technical correctness
    weight: 0.15
    auto: false
  - id: actionability
    name: Actionability
    weight: 0.10
    auto: false
  - id: prioritization
    name: Prioritization
    weight: 0.10
    auto: false
  - id: clarity
    name: Clarity
    weight: 0.05
    auto: false
```

## Judgement Workflow (Auto + Agent)

1. **Auto judgement**  
   - Uses completion status, output format hints, and constraint coverage.  
   - Produces `AutoJudgement` with dimension scores and a normalized score.

2. **Agent judgement**  
   - Runs only if rubric contains non-auto dimensions.  
   - Records dimension scores for correctness/actionability/prioritization/tradeoffs.

3. **Final outcome**  
   - Combines auto + agent scores by rubric weights.  
   - If auto fails (zero‑tolerance dimensions) → fail without agent judgement.  
   - If agent judgement missing → `needs_agent` status.

## Code Entry Points

- Eval set loader: `evaluation/agent_eval/eval_set_loader.go`
- Eval set definition/types: `evaluation/agent_eval/eval_set.go`
- Judging pipeline: `evaluation/agent_eval/judging.go`
- Baseline snapshots: `evaluation/agent_eval/baseline.go`

## Next Integration Targets

- Wire `EvalSetDefinition` + `JudgeRubric` into CLI/CI evaluation flows.  
- Persist `JudgementSummary` into evaluation reports and web dashboard.
