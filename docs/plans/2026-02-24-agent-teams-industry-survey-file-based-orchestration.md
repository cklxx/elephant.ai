# 2026-02-24 Agent Teams 行业调研 + File-Based 多 Agent 流程优化计划

## Status
- completed

## Goal
- 完成业界 agent teams 实现调研，形成利弊分析与本项目落地策略。
- 在本项目中补齐 file-based 团队执行记录与流程控制可观测性。
- 支持在团队编排中分配 `codex` / `claude_code` / `kimi` 作为执行角色。

## Scope
- 代码：
  - `internal/domain/agent/ports/agent/*`
  - `internal/domain/agent/react/*`
  - `internal/infra/tools/builtin/orchestration/*`
  - `internal/infra/external/*`
  - `internal/infra/coding/*`
  - `internal/shared/config/*`
  - `internal/app/di/*`
- 文档：
  - `docs/reference/CONFIG.md`
  - `docs/reference/external-agents-codex-claude-code.md`
  - `docs/research/*`

## Execution Steps
1. 设计并实现 team_dispatch file-based run recorder（自动记录 team run DAG 与 task 分配）。
2. 配置与执行链路新增 `kimi` external agent 支持（detect/config/registry/dispatch defaults）。
3. 补齐单元测试，验证 recorder 与 kimi 调度路径。
4. 联网调研主流 agent teams 实现，输出对比与本仓库优化建议。
5. 更新参考文档与示例 YAML，确保可直接落地。

## Progress Log
- 2026-02-24 23:58: 计划创建，完成现有实现基线扫描（team_dispatch/bg_plan/external bridge/config）。
- 2026-02-24 23:59: 打通 team run recorder 注入链路（React runtime/context、coordinator option、DI 构建）。
- 2026-02-24 23:59: `team_dispatch` 增加 file-based team run record 落盘与 `team_run_id/team_run_record_path` 元数据回传。
- 2026-02-25 00:00: 完成 `kimi` external agent 全链路支持（detect/config/registry/bridge/execution controls/DI auto-enable）。
- 2026-02-25 00:01: 补齐测试（team_dispatch recorder、teamrun file recorder、kimi detect/config/bridge/executor）并通过目标包测试。
- 2026-02-25 00:02: 完成业界 Agent Teams 调研文档与配置参考文档更新。
