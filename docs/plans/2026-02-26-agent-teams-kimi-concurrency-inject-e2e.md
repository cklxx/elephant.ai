# 2026-02-26 Agent Teams + Kimi CLI 并发注入式 E2E 验证计划

Created: 2026-02-26
Status: In Progress
Owner: Codex

## 背景

需要一个可重复、可自动化的验证 case，覆盖 `agent teams` 在真实执行链路下通过 `kimi cli` 执行多个并发任务的能力。现有测试覆盖了 team 模板渲染、并发调度与 bridge 执行，但缺少三者合并后的注入式端到端验证。

## 目标

新增一个 integration 级验证，覆盖：

- `run_tasks(template=...)` 触发 team workflow。
- 同一 stage 下多个 `kimi` 角色并发调度。
- 通过真实 `bridge.Executor` 调用注入的 fake `kimi` CLI（二进制注入）而非纯 mock。
- 验证状态文件、任务结果和并发性断言。

## 实施步骤

- [x] 梳理现有 team/taskfile/background/bridge 链路与测试空白。
- [x] 新增 integration 测试文件，构造 fake `kimi` CLI 注入点。
- [x] 通过 `run_tasks(template)` 驱动 team 执行，断言多并发 + kimi 请求参数。
- [x] 运行新增与相关测试。
- [x] 执行代码审查脚本并修复阻塞问题。
- [x] 提交增量 commit。

## 验收标准

1. 至少 3 个 `kimi` 角色在同一 stage 执行，断言 `max_active >= 2`。
2. 每个外部请求 `AgentType == "kimi"`。
3. `run_tasks(wait=true)` 返回成功且 `.status.yaml` 写入。
4. 所有 team 任务最终 `completed` 且有非空 answer。

## 进度记录

- 2026-02-26 16:40: 完成规范阅读、现有链路定位与测试策略确认。
- 2026-02-26 16:46: 新增 `agent_teams_kimi_inject_e2e_test.go`，完成 fake `kimi` 注入链路与 team template 并发断言。
- 2026-02-26 16:51: 通过 `go test ./internal/infra/integration -run TestAgentTeamsKimiInjectE2E_ParallelTemplate -count=1`。
- 2026-02-26 16:57: 通过回归：`go test ./internal/infra/tools/builtin/orchestration ./internal/domain/agent/react ./internal/infra/external/bridge ./internal/infra/integration -count=1`。
