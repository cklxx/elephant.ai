# Plan: 系统提示词优化（增强用户习惯记录）

**Date:** 2026-02-06  
**Status:** in_progress  
**Owner:** cklxx + Codex

## Goal
- 优化系统提示词，让 agent 在执行任务时更明确地识别并沉淀“用户习惯/偏好/固定工作方式”。
- 让自动记忆捕获（post-task memory capture）对“用户习惯”有更高提取优先级，减少只记录任务流水的情况。

## Non-goals
- 不新增 memory 存储后端或索引结构。
- 不引入新的工具或 memory 写入协议。
- 不改动 `agent/ports` 依赖边界。

## Approach
1. 在 `internal/app/context/manager_prompt.go` 增加习惯记录导向段落（系统提示词层）。
2. 在 `internal/app/agent/hooks/memory_capture.go` 强化 memory capture 提示词，明确优先提取用户习惯/偏好/稳定流程。
3. 使用 TDD 增补测试，覆盖提示词与捕获指令变化。
4. 运行全量 lint + tests，确认无回归。

## Milestones
- [x] 新增/更新系统提示词测试，先失败后实现
- [x] 实现系统提示词“用户习惯记录”指令
- [x] 新增/更新 memory capture 测试，先失败后实现
- [x] 实现 memory capture 对用户习惯的提取优先级
- [ ] 全量 lint + tests 通过（受仓库既有编译错误阻塞）

## Progress Log
- 2026-02-06 14: 已完成仓库规范准备（新 worktree、复制 `.env`、实践文档与记忆加载）。
- 2026-02-06 15: 先新增失败测试（system prompt + memory capture），再实现 Habit Stewardship 与习惯信号提取逻辑并回归通过相关测试。
- 2026-02-06 15: 执行 `./dev.sh lint` 与 `./dev.sh test`，被仓库现有 `internal/infra/tools/builtin/pathutil` 编译错误阻塞，非本次改动引入。
