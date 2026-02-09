# Foundation Systematic Eval Report Template

## 1. Run Metadata
- Run ID: `...`
- Suite Path: `evaluation/agent_eval/datasets/foundation_eval_suite.yaml`
- Generated At (UTC): `...`
- Commit: `...`

## 2. Aggregate x/x Scoreboard
- Collections: `x/x`
- Cases: `x/x`
- Applicable Cases: `x/x` (`N/A: x`)
- pass@1: `x/x`
- pass@5: `x/x`
- Deliverable Cases: `x/x`
- Deliverable Good: `x/x`
- Deliverable Bad: `x/x`
- Avg case latency p50/p95/p99 (ms): `x/x/x`
- Throughput (cases/s): `x`

## 3. Dimension Coverage x/x
- Base Tool Coverage: `x/x`
- Prompt Effectiveness Coverage: `x/x`
- Proactivity Coverage: `x/x`
- Complex High-Value Tasks: `x/x`
- Availability and Recovery: `x/x`
- Valuable Delivery Workflows: `x/x`
- SWE-bench Verified Readiness: `x/x`
- Multi-Step Orchestration: `x/x`
- Safety Boundary and Policy: `x/x`
- Context Learning Hard: `x/x`
- Memory Capabilities: `x/x`
- User Habit Soul Memory: `x/x`
- Task Completion Speed: `x/x`
- Long-Horizon Multi-Round: `x/x`
- Architecture Coding Hard: `x/x`
- Deep Research: `x/x`
- Autonomy Initiative: `x/x`
- Conflict Convergence Hard: `x/x`
- Challenge Hard V2: `x/x`
- Complex Artifact Delivery: `x/x`

## 4. Top1 Failure Cluster Inventory
| Conflict Pair (expected => top1) | Misses (x/x) | Miss Share | Collections | Sample Case |
|---|---:|---:|---:|---|
| `...` | `...` | `...` | `...` | `...` |

## 5. Failure Case Decomposition (Bad Cases)
| Collection | Case | Expected | Top1 | Hit Rank | Reason Signature | Action |
|---|---|---|---|---:|---|---|
| `...` | `...` | `...` | `...` | `...` | `...` | `...` |

## 6. Good Case Sampling Inspection
| Collection | Case | Expected | Top Matches | Why It Worked |
|---|---|---|---|---|
| `...` | `...` | `...` | `...` | `...` |

## 7. Deliverable Good/Bad Sampling
### Good Cases
| Collection | Case | Contract (matched/required) | Coverage | Why Good |
|---|---|---:|---:|---|
| `...` | `...` | `...` | `...` | `...` |

### Bad Cases
| Collection | Case | Contract (matched/required) | Coverage | Why Bad |
|---|---|---:|---:|---|
| `...` | `...` | `...` | `...` | `...` |

## 8. Systematic Optimization Plan
1. Conflict family A: expected `...` vs top1 `...`
   - Root cause:
   - Router/token fix:
   - Dataset hardening:
   - Regression tests:
2. Conflict family B: ...

## 9. Next-Round Targets
- Current pass@1: `x/x`
- Target pass@1: `x/x`
- Retire saturated easy cases: `...`
- Inject new hard cases: `...`
