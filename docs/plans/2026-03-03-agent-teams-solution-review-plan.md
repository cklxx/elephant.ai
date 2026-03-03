# 2026-03-03 Agent Teams 方案审查计划

Updated: 2026-03-03
Status: Completed

## 目标
- 对当前 `agent teams` 方案做一次端到端审查。
- 输出一份可用于后续演进的详细梳理文档（架构、数据流、配置、测试、风险、改进建议）。

## 范围
- Kernel team dispatch（planner/executor/engine）
- `run_tasks` / `reply_agent` orchestration 工具
- `taskfile` 执行模型（team/swarm/auto）
- BackgroundTaskManager 依赖、上下文继承、输入回传
- `external_agents.teams` 配置加载与 DI 注入
- Team run recorder 与状态 sidecar
- 文档与测试覆盖一致性

## 执行步骤
1. 基线确认（pre-work checklist）
- `git diff --stat`
- `git log --oneline -10`
- 识别并隔离当前工作区无关改动，避免覆盖。

2. 规范与经验加载
- 阅读 `docs/guides/engineering-practices.md`
- 阅读 `docs/guides/documentation-rules.md`
- 阅读 `docs/guides/orchestration.md`
- 阅读 `docs/guides/agents-teams-testing.md`
- 加载近期 error/good 经验条目与 summary。

3. 代码与测试审查
- Kernel: `types/config/planner/executor/engine`
- Orchestration: `run_tasks/reply_agent`
- Taskfile: validate/resolve/mode/executor/swarm/status/template/topo
- React runtime/background 管理器与 external input 路径
- 配置链路：`types/file_config/runtime_file_loader/load`
- DI：`container_builder/builder_hooks`
- e2e/integration: `internal/infra/integration/agent_teams_*` 与脚本

4. 产出与索引
- 新增 `docs/reviews/2026-03-03-agent-teams-solution-full-review.md`
- 补齐 `docs/reviews/README.md` 索引并纳入新文档。

5. 交付
- 仅提交本任务相关文档文件。

## 完成情况
- [x] 基线确认
- [x] 规范与经验加载
- [x] 代码与测试审查
- [x] 详细 review 文档输出
- [x] `docs/reviews` 目录索引补齐

## 结果补充（第二轮整理）
- 已按用户要求重构 review 结论：仅保留“文档与脚本漂移”为现存问题。
- 其余项已改写为“优化设计方案”，并给出可执行的落地优先级。
