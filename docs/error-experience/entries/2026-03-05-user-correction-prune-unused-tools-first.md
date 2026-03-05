# 2026-03-05 · 用户明确“工具没用要删”但未先固化为工具面收敛规则

## Context
- 任务目标：梳理当前内置工具并继续做工具层优化。
- 用户纠正：`plan` 没用，要求删除；并将 `run_tasks/reply_agent` 抽象为 CLI，放到 skills 说明使用方式。

## Symptom
- 收到“工具没用要删”的明确指令后，没有第一时间将其固化成“先收敛工具面，再做迁移”的强规则。

## Root Cause
- 在进入实现前，缺少“用户纠正优先级高于既有设计偏好”的显式落地步骤。
- 对工具迁移任务的执行顺序未固定，容易先做实现细节再做工具面收敛。

## Remediation
- 收到用户对工具价值的明确纠正后，必须先执行：
  - 把纠正写为不可违反约束（例如：`plan` 直接下线，不保留软兼容入口）。
  - 先完成注册面收敛（registry/preset/prompt），再做 CLI 抽象与文档迁移。
  - 最后删除失效代码与失效文档，避免“功能已迁移但旧工具仍可调用”。

## Follow-up
- 后续凡是“工具下线 + 能力迁移”任务，统一采用顺序：
  1) 工具注册面下线；
  2) CLI/技能替代能力上线；
  3) 失效实现与文档清理。

## Metadata
- id: err-2026-03-05-user-correction-prune-unused-tools-first
- tags: [user-correction, tools, deprecation, migration]
- links:
  - docs/error-experience/summary/entries/2026-03-05-user-correction-prune-unused-tools-first.md
