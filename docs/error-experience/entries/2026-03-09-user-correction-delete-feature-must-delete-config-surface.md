# 2026-03-09 · 用户要求删除功能时必须同步删除对应配置面

## Context
- 用户补充要求：“以及要删除好 config 的配置”。
- 原始实现里 `kernel` 不只是代码目录，还散落在 bootstrap、CLI、健康检查、提示注入、脚本与注释等配置邻接面。

## Symptom
- 如果只删除运行时代码，会留下失效配置、子命令、健康探针或注释，形成伪能力与维护噪音。

## Root Cause
- 过早把“删功能”理解为仅删核心包，没有把配置面、入口面、可观测性和提示注入视为同一功能面的组成部分。

## Remediation
- 收到“删除某功能/模块”且用户补充“配置也删掉”时，默认同步检查并清理：
  - 运行时代码与依赖注入
  - 配置结构、默认值、热加载入口
  - CLI/脚本/守护进程入口
  - 健康检查、通知、监控与文档注释中的活跃承诺
- 验证标准不是“核心包已删”，而是“无 live reference、无 dead config surface”。

## Follow-up
- 将“功能删除 = 代码面 + 配置面 + 入口面一起收口”作为默认删除策略。

## Metadata
- id: err-2026-03-09-user-correction-delete-feature-must-delete-config-surface
- tags: [user-correction, deletion-discipline, config-surface]
- links:
  - docs/error-experience/summary/entries/2026-03-09-user-correction-delete-feature-must-delete-config-surface.md
