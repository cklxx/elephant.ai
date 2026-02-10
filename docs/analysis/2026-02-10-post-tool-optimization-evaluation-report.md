# 2026-02-10 工具大改后评测复盘（Post Tool Optimization Evaluation）

## 1. 执行范围
- 评测日期：2026-02-10
- 目标：验证“工具大规模优化”后的实际能力变化
- 套件：
  - `evaluation/agent_eval/datasets/foundation_eval_suite_e2e_systematic.yaml`
  - `evaluation/agent_eval/datasets/foundation_eval_suite.yaml`
- 产物：
  - `tmp/foundation-suite-r20-e2e-20260210-114913`
  - `tmp/foundation-suite-r20-current-20260210-114913`

## 2. 结果总览（x/x）

### 2.1 E2E Systematic Suite
- Collections: `25/28`
- Cases: `181/186`（Applicable）
- N/A: `177`
- pass@1: `162/186`
- pass@5: `181/186`
- Failed: `5`
- Deliverable Good: `0/20`

### 2.2 Current Main Suite
- Collections: `26/28`
- Cases: `129/131`（Applicable）
- N/A: `138`
- pass@1: `110/131`
- pass@5: `129/131`
- Failed: `2`
- Deliverable Good: `0/30`

## 3. 与上一轮基线对比（2026-02-10 早先 run）

### 3.1 E2E Suite 对比
- 之前：`pass@1 312/363`，`pass@5 361/363`，`Failed 3`，`N/A 0`，`Deliverable Good 14/20`
- 当前：`pass@1 162/186`，`pass@5 181/186`，`Failed 5`，`N/A 177`，`Deliverable Good 0/20`

### 3.2 Current Suite 对比
- 之前：`pass@1 224/269`，`pass@5 269/269`，`Failed 0`，`N/A 0`，`Deliverable Good 24/30`
- 当前：`pass@1 110/131`，`pass@5 129/131`，`Failed 2`，`N/A 138`，`Deliverable Good 0/30`

结论：本轮不是“语义微退化”，而是“工具可用性覆盖断崖式下降”。

## 4. 根因定位

评测期识别到的可用工具仅 `14` 个：
- `browser_action`
- `channel`
- `clarify`
- `execute_code`
- `memory_get`
- `memory_search`
- `plan`
- `read_file`
- `replace_in_file`
- `request_user`
- `shell_exec`
- `skills`
- `web_search`
- `write_file`

缺失了大量核心评测依赖工具（如 `find`、`list_dir`、`search_file`、`ripgrep`、`artifacts_write`、`artifacts_list`、`artifact_manifest`、`lark_*`、`write_attachment` 等），直接导致：
- 大量 case 被判定为 `availability_error` / `N/A`
- deliverable 检查全面失效（Good 归零）
- 失败 case 主要由 availability 主导而非推理失误

## 5. 失败样例（Top）
- `agentbench-hard-cli-evidence-run`：`shell_exec` 期望，Top 命中偏离并跌出 Top-K。
- `tau2-hard-persist-exec-decision-record`：`artifacts_write` 不可用（availability_error）。
- `tau2-hard-thread-history-for-latest-constraint`：`lark_chat_history` 不可用（availability_error）。
- `agentlongbench-hard-repo-state-topology-refresh`：`find` 不可用（availability_error）。

## 6. 对当前 Agent 交付能力的判断
1. 现阶段评测分数无法代表真实“智能能力提升”，因为主要瓶颈是工具可用性覆盖丢失。  
2. 在恢复工具注册/别名/可见性之前，任何 pass@1 优化结论都不可信。  
3. 下一步必须先修复工具可用性，再进行语义路由和提示词优化。  

## 7. 建议的修复顺序
1. 恢复评测关键工具在 `web/full/default` 下的注册覆盖（至少覆盖历史基线工具集）。  
2. 对“工具可用性清单”加自动化守护：评测前先做 tool inventory diff。  
3. 回归标准：先把 `N/A` 压回到接近 `0`，再看 pass@1/pass@5 改善。  
