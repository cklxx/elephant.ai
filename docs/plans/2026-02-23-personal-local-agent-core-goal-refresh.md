# 2026-02-23 Personal Local Agent Core Goal Refresh Plan

**Created**: 2026-02-23
**Status**: In Progress
**Branch**: feat/local-agent-core-goal-20260223
**Owner**: Codex + cklxx

## Objective

将 elephant.ai 的核心目标统一更新为：
- 构建“个人本地 agent”作为单人智能杠杆器。
- 以“主动性 + 上下文压缩”降低用户注意力负担。
- 在用户可覆盖前提下，最大化单人可调用的模型智能与交付吞吐。

## North-Star Requirements

1. 注意力节省（Attention Saving）
- 默认输出最小必要信息、可执行下一步、低噪声状态回报。
- 通过摘要/压缩保持长会话可持续，不让上下文膨胀拖垮决策质量。

2. 判断力杠杆（Judgment Leverage）
- 机器负责探索、整理、执行；人类聚焦目标、约束、最终判断。
- 让单人可并行驱动多个 subagent、并在统一上下文下收敛结果。

3. 主动但可控（Proactive but Overrideable）
- 主动提出澄清、计划、提醒、执行建议，但保持显式可覆盖。
- 严禁操控式语言；敏感/不可逆动作必须征求同意。

## Scope

- 全仓代码与关键文档进行一致性审计：
  - `cmd/`, `internal/`, `web/`, `scripts/`, `tests/`, `configs/`, `docs/`。
- 找出“符合点 / 不符合点 / 优化点”，形成可追踪清单。
- 先落地核心路径改造（目标定义、系统提示、子代理协作、上下文压缩策略），再逐步扩展。

## Constraints & Standards

- 保持架构边界：`agent/ports` 不引入 memory/RAG 依赖。
- 非平凡逻辑改动遵循 TDD；提交前跑全量 lint + tests。
- 采用增量提交；每个提交保持可回滚和可审查。
- 参考最佳实践：ReAct/Toolformer 类代理编排、Google SRE 可观测性基线、Go Code Review Comments、OWASP 安全最小权限原则。

## Active Memory (Selected)

1. `agent/ports` 必须保持无 memory/RAG 依赖，防止循环依赖。
2. 配置示例仅使用 `.yaml`。
3. 逻辑改动优先 TDD，交付前跑全量 lint+tests。
4. subagent 事件分组以 `parent_run_id` 为主，防止 UI 归组碎片化。
5. 仅 subagent 的 `workflow.tool.started` 进入主流，其他工具进入 pending/merged。
6. 大规模重构前先守住 tool availability 基线，避免“可用性塌陷”掩盖能力评估。
7. 长生命周期 map/会话指标必须有 cap + TTL，避免无界增长。
8. Kernel/主动循环默认低频稳态，避免共享配额突发限流。
9. 交付类意图需显式区分“仅文本”与“必须文件交付”。
10. 预推送 gate 与 CI 快速失败项保持同构。

## Execution Plan

### Phase 0 — Baseline & Inventory
- [x] 新建 worktree 分支并复制 `.env`。
- [x] 加载工程实践与记忆条目，形成 active memory。
- [x] 统计全仓代码文件清单（按目录和语言分类）。

### Phase 1 — Parallel Subagent Audit (Full Repo)
- [x] 用多个 subagent 并行审计：
  - A: `internal/agent`, `internal/app`, `internal/domain`
  - B: `internal/tools`, `internal/memory`, `internal/llm`, `internal/channels`
  - C: `web/`, `cmd/`
  - D: `scripts/`, `tests/`, `configs/`, `docs/`
- [x] 产出统一审计表：每个子域给出 `符合/不符合/优化建议` + 证据文件路径。

### Phase 2 — Core Goal Refactor (First Wave)
- [x] 更新核心目标文档（README/ROADMAP/关键设计文档）为“个人本地 agent 智能杠杆”。
- [x] 更新系统提示/策略配置：强化“主动性 + 上下文压缩 + 注意力节省”。
- [x] 更新 subagent 协作策略：强调并行探索、收敛总结、用户可覆盖。
- [x] 对关键代码路径做最小必要改造并补测试。

### Phase 3 — Validation & Review
- [x] 执行全量 lint + 全量测试。
- [x] 执行强制代码评审流程（`skills/code-review/SKILL.md` 7 步），输出 P0-P3 报告。
- [x] 修复评审问题并回归。

### Phase 4 — Delivery Hygiene
- [ ] 增量提交（按主题拆分多个 commits）。
- [ ] merge 回 `main`（优先 fast-forward）。
- [ ] 删除临时 worktree，清理分支。
- [ ] 更新本计划状态与执行日志。

## Acceptance Criteria

1. 核心目标在项目主文档与关键策略入口处一致表达。
2. 已完成全仓并行审计并形成“符合/不符合/优化”证据清单。
3. 至少一轮关键路径代码已改造并通过测试。
4. lint + tests 全绿，评审无 P0/P1 残留。
5. 改动以多次增量提交落地并完成主干合并。

## Progress Log

### 2026-02-23 22:43 — Plan initialized
- 已创建独立 worktree：`/Users/bytedance/code/elephant.ai.worktrees/local-agent-core-goal-20260223`
- 已复制 `.env`。
- 已加载工程实践与记忆摘要，完成 active memory 选择。
- 下一步：启动多 subagent 全仓审计并生成统一差距报告。

### 2026-02-23 23:05 — Full-repo audit completed
- 已并行运行 4 个 subagent 审计分区（core internal / tools+memory / web+cmd / scripts+tests+configs+docs）。
- 已统计全仓代码文件清单（2712 个 code-ish 文件）并按目录、后缀分类。
- 已产出审计报告：`docs/reviews/2026-02-23-personal-local-agent-alignment-audit.md`。

### 2026-02-23 23:24 — Core refactor + tests completed
- 已更新核心目标与路线图文档：`README.md`, `README.zh.md`, `ROADMAP.md`, `docs/roadmap/roadmap.md`。
- 已更新策略/人格入口：`configs/context/policies/default.yaml`, `docs/reference/SOUL.md`, `internal/domain/agent/presets/prompts.go`。
- 已落地代码改造：
  1) `BackgroundTaskManager` 增加并发上限控制。
  2) `TrimMessages` 与 `EstimateMessageTokens` 统一计数口径。
- 已补充回归测试：`internal/domain/agent/react/background_test.go`, `internal/app/context/trimmer_test.go`。

### 2026-02-23 23:44 — Validation + code review completed
- 已通过全量质量闸门：`./scripts/pre-push.sh`（含 go test -race / lint / arch / web lint+build）。
- 已执行强制代码评审并输出报告：`docs/reviews/2026-02-23-core-goal-refresh-code-review.md`（P0/P1=0）。
