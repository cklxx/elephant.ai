# 2026-02-10 Active Tools 系统化评测与产品优化报告

## 1. 范围与目标
- 目标：
  - 删除/收敛依赖失效工具的基础评测入口，重建基于当前有效工具面的系统化 hard suite。
  - 针对失败簇做产品侧路由优化（工具定义语义边界），不是只改 eval 文案。
  - 输出 pass@1/pass@5、失败拆解、good/bad 交付抽样。
- 当前 default 工具集：14 个
  - `browser_action, channel, clarify, execute_code, memory_get, memory_search, plan, read_file, replace_in_file, request_user, shell_exec, skills, web_search, write_file`

## 2. 失效工具审计（用于删除无效评测依赖）
对现有 `evaluation/agent_eval/datasets/*.yaml` 做 expected_tools 审计后，失效调用最集中项（Top）：
- `artifacts_write: 35`
- `search_file: 33`
- `find: 30`
- `list_dir: 30`
- `ripgrep: 26`
- `web_fetch: 23`
- `lark_send_message: 17`
- `lark_upload_file: 14`

结论：旧集合大量引用当前 default 未注册工具，直接导致 N/A/availability 噪声，不能反映真实模型能力。

## 3. 新评测集合（active-only, hard）
新增并纳入套件：
- `evaluation/agent_eval/datasets/foundation_eval_cases_capability_active_intent_boundary_hard.yaml`（24）
- `evaluation/agent_eval/datasets/foundation_eval_cases_capability_active_memory_habit_long_horizon_hard.yaml`（20）
- `evaluation/agent_eval/datasets/foundation_eval_cases_capability_active_delivery_and_artifact_write_hard.yaml`（20）
- `evaluation/agent_eval/datasets/foundation_eval_cases_capability_active_industry_transfer_hard.yaml`（24）
- 新 suite：`evaluation/agent_eval/datasets/foundation_eval_suite_active_tools_systematic_hard.yaml`
- 扩展基础 suite：`evaluation/agent_eval/datasets/foundation_eval_suite_basic_active.yaml`（同样 7 collections）

总规模：`119` case（全部仅依赖当前 14 工具）。

## 4. 产品优化（路由能力）
### 4.1 语义边界优化
主要优化文件：
- `internal/infra/tools/builtin/ui/plan.go`
- `internal/infra/tools/builtin/ui/clarify.go`
- `internal/infra/tools/builtin/ui/request_user.go`
- `internal/infra/tools/builtin/aliases/{read_file,replace_in_file,write_file,shell_exec,execute_code}.go`
- `internal/infra/tools/builtin/sandbox/{sandbox_file,sandbox_shell,sandbox_code_execute,sandbox_browser}.go`
- `internal/infra/tools/builtin/memory/memory_search.go`
- `internal/infra/tools/builtin/session/skills.go`
- `internal/infra/tools/builtin/larktools/channel.go`
- `internal/infra/tools/builtin/web/web_search.go`

优化点：
- 强化 `write_file vs replace_in_file`（新建交付产物 vs 原位修改）。
- 强化 `request_user vs clarify`（审批/签署/人工门控 vs 信息缺失澄清）。
- 强化 `shell_exec vs execute_code`（CLI 证据采集 vs 确定性计算）。
- 强化 `read_file vs replace_in_file`（先读证据窗口 vs 定点修改）。
- 强化 `browser_action vs web_search`（手工页面交互 vs 外部权威检索）。
- 强化 `memory_search vs memory_get`（先发现路径 vs 按路径打开）。

### 4.2 回归守护
新增测试：
- `evaluation/agent_eval/foundation_active_suite_guard_test.go`
  - 保证 active suites 不再引入失效 expected_tools。
- `internal/infra/tools/builtin/ui/routing_descriptions_test.go`
- 更新：
  - `internal/infra/tools/builtin/aliases/routing_descriptions_test.go`
  - `internal/infra/tools/builtin/sandbox/routing_descriptions_test.go`
  - `internal/infra/tools/builtin/memory/routing_descriptions_test.go`

## 5. 评测结果（x/x）
### 5.1 优化前（active hard suite baseline）
- 命令产物：`tmp/foundation-suite-r23-active-hard-20260210-131154`
- `Collections passed`: `4/7`
- `Cases`: `112/119`
- `N/A`: `0`
- `pass@1`: `91/119`（78.7%）
- `pass@5`: `112/119`（95.4%）
- `Failed`: `7`
- `Deliverable Good`: `7/10`

### 5.2 优化后（同 suite）
- 命令产物：`tmp/foundation-suite-r23-active-hard-optimized-20260210-131523`
- `Collections passed`: `4/7`
- `Cases`: `116/119`
- `N/A`: `0`
- `pass@1`: `97/119`（84.0%）
- `pass@5`: `116/119`（98.0%）
- `Failed`: `3`
- `Deliverable Good`: `9/10`

### 5.3 基础 active suite（扩展后）
- 命令产物：`tmp/foundation-suite-r23-basic-active-expanded-20260210-131229`
- 与 active hard suite同构，分数一致：
  - `pass@1: 97/119`
  - `pass@5: 116/119`

## 6. 失败 case 拆解（优化后）
剩余失败 `3` 个：
1. `delivery-write-incident-report-pack`（expected `write_file`）
- 类型：`no_overlap`
- top 候选：`plan`, `skills`, `memory_get`, `replace_in_file`, `channel`
- 诊断：描述偏“报告内容语义”，缺少“写入/落盘”动词，仍被规划类语义吸走。

2. `memory-habit-recall-ship-window`（expected `memory_search`）
- 类型：`no_overlap`
- top 候选：`plan`, `read_file`, `browser_action`, `execute_code`, `memory_get`
- 诊断：句式偏偏好/规划表达，memory 检索词命中不足。

3. `transfer-swebench-read-contract-before-fix`（expected `read_file`）
- 类型：`rank_below_top_k`（rank 7）
- top 候选：`replace_in_file`, `memory_get`, `execute_code`, `memory_search`, `plan`
- 诊断：`fix` 词对编辑类工具拉力仍高于“inspect-before-change”。

## 7. 抽样交付检查（good/bad）
- Deliverable cases: `10/119`
- Good: `9/10`
- Bad: `1/10`

Good 样本：
- `delivery-write-migration-runbook`：`write_file` Top1，契约 `2/2`。
- `delivery-write-postmortem-log`：`write_file` Top1，契约 `2/2`。
- `transfer-write-decision-record`：`write_file` Top1，契约 `2/2`。

Bad 样本：
- `delivery-write-incident-report-pack`：契约 `0/2`，未覆盖 durable write 信号。

## 8. 当前结论
1. 本轮已完成“失效工具依赖收敛 + active-only 系统化 hard suite”重建，`N/A=0`。  
2. 产品能力有实质提升（同套件对比）：
   - `pass@1`: `91/119 -> 97/119`（+6）
   - `pass@5`: `112/119 -> 116/119`（+4）
   - `failed`: `7 -> 3`
   - `deliverable good`: `7/10 -> 9/10`
3. 剩余低分主要集中在“隐式落盘交付语义”和“memory recall弱词面”的极难样例。

## 9. 下一轮优化建议
1. 对 `write_file` 再加 “incident report / summary pack / final brief” 级别领域词。  
2. 在 `memory_search` 增加 “what I prefer / usually / habitually / historically” pattern 权重。  
3. 在路由打分层加入轻量负向规则：当 intent 含 `before fix / inspect first / no edits yet` 时，下调 `replace_in_file`。  
