# Plan: Runtime config 更新后 agent 不生效（接入 runtime cache per-task）

## Status: Completed
## Date: 2026-02-04

## Problem
- 本地更新 `~/.alex/test.yaml`（或 `ALEX_CONFIG_PATH` 指向的配置）后，运行中的 `alex-server` agent 行为/配置（如 provider/model）未变化。

## Hypothesis
- Server 启动时把 `RuntimeConfig` 映射成 DI/Coordinator 的静态 `appconfig.Config`。
- 虽然 bootstrap 已经有 `RuntimeConfigCache` + fsnotify watcher 触发 reload，但 agent 执行路径没有使用 cache 的最新快照（仅 config API 使用 resolver）。

## Goals
- 配置文件变更后（watcher reload 触发），**后续新任务**使用最新 runtime config（不要求中途切换正在执行的任务）。
- 失败时不阻塞执行：配置解析/加载失败时继续用上一次可用配置（或启动时配置）。

## Plan
1. 追踪 `alex-server` 执行链路中 runtime config 的使用点（Coordinator/Preparation）。
2. 给 `AgentCoordinator` 注入 `RuntimeConfigResolver`（来自 `RuntimeConfigCache.Resolve`），在每次 task 开始时获取快照并用于 Prepare + ReAct 配置。
3. 补单测：验证 resolver 返回的新 provider/model 会被用于 `PrepareExecution` 初始化 LLM client。
4. 跑全量 `./dev.sh lint && ./dev.sh test`。

## Progress
- [x] Trace config usage in agent execution path.
- [x] Wire runtime resolver into coordinator per-task config.
- [x] Add regression tests.
- [x] Run full lint + tests.
