# Agent Real E2E Evaluation Report (2026-02-13)

## Run Context
- Date: 2026-02-13
- Workspace: `elephant.ai.worktrees/agent-e2e-eval-20260213`
- Mode: real local execution via `go run ./cmd/alex eval foundation-suite ...`
- Outputs:
  - `tmp/foundation-suite-e2e-systematic-real-20260213-012415`
  - `tmp/foundation-suite-current-real-20260213-012512`
  - `tmp/foundation-suite-basic-active-real-20260213-012528`
  - `tmp/foundation-suite-motivation-aware-real-20260213-012813`
  - `tmp/foundation-suite-active-tools-hard-real-20260213-012818`

## Commands
```bash
go run ./cmd/alex eval foundation-suite \
  --suite evaluation/agent_eval/datasets/foundation_eval_suite_e2e_systematic.yaml \
  --output tmp/foundation-suite-e2e-systematic-real-20260213-012415 \
  --format markdown

go run ./cmd/alex eval foundation-suite \
  --suite evaluation/agent_eval/datasets/foundation_eval_suite.yaml \
  --output tmp/foundation-suite-current-real-20260213-012512 \
  --format markdown

go run ./cmd/alex eval foundation-suite \
  --suite evaluation/agent_eval/datasets/foundation_eval_suite_basic_active.yaml \
  --output tmp/foundation-suite-basic-active-real-20260213-012528 \
  --format markdown

go run ./cmd/alex eval foundation-suite \
  --suite evaluation/agent_eval/datasets/foundation_eval_suite_motivation_aware.yaml \
  --output tmp/foundation-suite-motivation-aware-real-20260213-012813 \
  --format markdown

go run ./cmd/alex eval foundation-suite \
  --suite evaluation/agent_eval/datasets/foundation_eval_suite_active_tools_systematic_hard.yaml \
  --output tmp/foundation-suite-active-tools-hard-real-20260213-012818 \
  --format markdown
```

## Headline Metrics

### 1) E2E Systematic Suite
- Suite: `foundation_eval_suite_e2e_systematic.yaml`
- Collections: `27/28`
- Cases: `199/202` (applicable), total `344`
- pass@1: `179/202` (`85.5%`)
- pass@5: `200/202` (`96.0%`)
- Failed cases: `3`
- Availability errors: `142`
- Deliverable good/bad: `0/20`, `20/20`
- Artifact:
  - `tmp/foundation-suite-e2e-systematic-real-20260213-012415/foundation_suite_result_foundation-suite-20260212-172425.json`
  - `tmp/foundation-suite-e2e-systematic-real-20260213-012415/foundation_suite_report_foundation-suite-20260212-172425.md`

### 2) Current Main Suite
- Suite: `foundation_eval_suite.yaml`
- Collections: `27/28`
- Cases: `130/131` (applicable), total `269`
- pass@1: `112/131` (`82.7%`)
- pass@5: `130/131` (`92.4%`)
- Failed cases: `1`
- Availability errors: `138`
- Deliverable good/bad: `0/30`, `30/30`
- Artifact:
  - `tmp/foundation-suite-current-real-20260213-012512/foundation_suite_result_foundation-suite-20260212-172514.json`
  - `tmp/foundation-suite-current-real-20260213-012512/foundation_suite_report_foundation-suite-20260212-172514.md`

### 3) Basic Active Suite
- Suite: `foundation_eval_suite_basic_active.yaml`
- Collections: `4/7`
- Cases: `116/120` (applicable), total `120`
- pass@1: `97/120` (`83.3%`)
- pass@5: `116/120` (`97.3%`)
- Failed cases: `4`
- Availability errors: `0`
- Deliverable good/bad: `9/10`, `1/10`
- Artifact:
  - `tmp/foundation-suite-basic-active-real-20260213-012528/foundation_suite_result_foundation-suite-20260212-172528.json`
  - `tmp/foundation-suite-basic-active-real-20260213-012528/foundation_suite_report_foundation-suite-20260212-172528.md`

### 4) Motivation-Aware Suite
- Suite: `foundation_eval_suite_motivation_aware.yaml`
- Collections: `1/1`
- Cases: `10/10` (applicable), total `17`
- pass@1: `10/10` (`100.0%`)
- pass@5: `10/10` (`100.0%`)
- Failed cases: `0`
- Availability errors: `7`
- Deliverable good/bad: `0/0`, `0/0`（no deliverable cases）
- Artifact:
  - `tmp/foundation-suite-motivation-aware-real-20260213-012813/foundation_suite_result_foundation-suite-20260212-172813.json`
  - `tmp/foundation-suite-motivation-aware-real-20260213-012813/foundation_suite_report_foundation-suite-20260212-172813.md`

### 5) Active Tools Systematic Hard Suite
- Suite: `foundation_eval_suite_active_tools_systematic_hard.yaml`
- Collections: `4/7`
- Cases: `116/120` (applicable), total `120`
- pass@1: `97/120` (`83.3%`)
- pass@5: `116/120` (`97.3%`)
- Failed cases: `4`
- Availability errors: `0`
- Deliverable good/bad: `9/10`, `1/10`
- Artifact:
  - `tmp/foundation-suite-active-tools-hard-real-20260213-012818/foundation_suite_result_foundation-suite-20260212-172818.json`
  - `tmp/foundation-suite-active-tools-hard-real-20260213-012818/foundation_suite_report_foundation-suite-20260212-172818.md`

## Failure Focus

### E2E Systematic: failed collection
- Collection: `user-habit-soul-memory`
- Failed cases: `3` (collection summary), from case results failed IDs observed:
  - `soul-memory-manifest-proof`
  - `habit-reminder-rhythm`
  - `habit-task-assignment-style`
  - `habit-calendar-pattern`
- Pattern: memory/habit intent routing still偏向通用工具（`skills` / `browser_action` / `channel`）而非专用期望路径。

### Current Main: failed collection
- Collection: `industry-benchmark-long-context-reasoning`
- Failed cases: `1`
- Pattern: 长上下文推理映射仍存在 top1 偏差。

### Basic Active: failed collections
- `capability-active-memory-habit-long-horizon-hard` (`1`)
- `capability-active-delivery-and-artifact-write-hard` (`2`)
- `capability-active-industry-transfer-hard` (`1`)
- Pattern: active tool surface可用性正常（`N/A=0`），但 hard intent mapping 仍有局部冲突。

### Motivation-Aware: no semantic failures, but availability gap
- Collection: `motivation-aware-proactivity`
- Failed cases: `0`
- Availability errors: `7`
- Pattern: motivation-aware 路由语义正确，但可用性缺口仍会造成不可执行 case（`N/A`）。

### Active Tools Systematic Hard: failed collections and case-level failures
- Failed collections:
  - `capability-active-memory-habit-long-horizon-hard` (`1`)
  - `capability-active-delivery-and-artifact-write-hard` (`2`)
  - `capability-active-industry-transfer-hard` (`1`)
- Failed cases:
  - `memory-habit-recall-ship-window`
  - `delivery-write-incident-report-pack`
  - `delivery-request-user-final-signoff`
  - `transfer-swebench-read-contract-before-fix`
- Pattern:
  - `request_user => clarify`
  - `read_file => replace_in_file`
  - `memory_search` 在 long-horizon habit case 上出现 no-overlap

## Comparison Notes
- e2e systematic 与历史基线形态一致：高 `pass@5` + 低 `deliverable good` + 高 availability error。
- current main suite 的 applicable case 更少，失败数更低，但对复杂交付与 frontier transfer 压力覆盖不如 e2e systematic。
- basic active suite 是当前最能反映“可用工具面健康度”的入口：`N/A=0`、deliverable 指标显著更好。
- motivation-aware suite 语义已稳定（`100%`），但 availability 仍需和 active tool 注册能力对齐。
- active-tools-hard 与 basic-active 指标一致（同为 `116/120`），可作为 hard routing 回归的稳定对照基线。

## Recommended Next Actions
1. 针对 `user-habit-soul-memory` 与 `active-tools-hard` 失败簇加 intent-level 回归（优先修 `request_user => clarify`、`read_file => replace_in_file`）。
2. 在 `memory_search`/habit-long-horizon 路由上补充词面覆盖和惩罚项，消除 `no_overlap` 类型失败。
3. 针对 deliverable 维度继续做“文本-only vs file-delivery”精细分流，提升 `deliverable_good_ratio`。
4. 将本次五套 run 作为 2026-02-13 回归基线，后续变更统一对比这五组路径。
