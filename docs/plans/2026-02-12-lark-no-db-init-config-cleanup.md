# 2026-02-12 — Lark 无 DB 依赖 + 配置清理 + setup 新手初始化

Date: 2026-02-12
Owner: cklxx + Codex
Status: in_progress

## Context
- 目标是让 Lark 模式默认不依赖 Postgres/AuthDB，并将状态存储改为 file/memory。
- 同时清理 Lark 无效配置项，减少用户手工配置负担。
- 扩展 `alex setup`，让新手一次完成必要配置选择（运行模式 + Lark 关键项 + 持久化模式）。

## Decisions (Locked)
- 持久化策略：`file` 优先，支持 `memory`。
- 兼容策略：本次直接硬删无效/遗留配置字段。
- CLI 入口：扩展 `alex setup`（不新增顶级命令）。
- 范围：包含 `alex-server` 与 `alex dev up --lark` / `alex dev lark` 的编排层。

## Work Items
1. 配置模型：新增 `channels.lark.persistence.*`，移除 `task_store_enabled` 等遗留字段读取。
2. Lark 存储：新增 Task/PlanReview/ChatSessionBinding 的 file 与 memory store。
3. Bootstrap：Lark gateway 统一按 persistence mode 装配存储，移除对 `SessionDB` 的硬依赖路径。
4. Dev 编排：`dev up --lark` 默认不再启动 `authdb`，可通过显式 flag 覆盖。
5. Setup 向导：扩展 `alex setup` 交互流程，覆盖运行模式、Lark 必要项与持久化模式。
6. 文档与测试：更新 CONFIG/README，补充单测与集成回归。
7. 代码审查：按 `skills/code-review/SKILL.md` 产出分级报告并修复。

## Progress Log
- 2026-02-12 14:20: 创建执行计划，完成现状勘察与改造边界确认。
- 2026-02-12 14:35: 引入 `channels.lark.persistence.*`，并在 bootstrap 增加 mode/dir/retention/max_tasks 校验。
- 2026-02-12 14:48: 新增 Lark 本地存储实现（task/plan_review/chat_session 的 file + memory）。
- 2026-02-12 14:55: Lark gateway 改为按 persistence mode 装配本地存储，移除对 SessionDB 的依赖路径。
- 2026-02-12 15:05: `alex dev up --lark` 默认跳过 authdb，增加 `--with-authdb` 显式开关。
- 2026-02-12 15:18: 扩展 `alex setup` 为新手初始化向导，增加运行模式/Lark 凭据/持久化模式配置写入。
