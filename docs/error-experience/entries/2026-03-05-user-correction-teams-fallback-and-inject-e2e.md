# 2026-03-05 · 将 teams 测试误做成单点冒烟，遗漏 fallback 与注入端到端

## Context
- 任务目标：验证 teams 功能可用性，覆盖自动 fallback 与注入能力。
- 用户纠正：测试不应依赖手工指定 provider，应验证 teams 自动 fallback；并补注入功能的端到端效果。

## Symptom
- 初始测试偏向单点冒烟（单 provider、单调用），没有覆盖：
  - `fallback_clis` 自动生效链路；
  - `run_tasks` 运行中通过 `reply_agent(message)` 注入的产品路径。

## Root Cause
- 将“能跑通一次”误当成“teams 功能验证完成”。
- 对 teams 的核心验收标准（fallback + inject）没有先固化为测试清单。

## Remediation
- 固化 teams 验收基线：
  1) 必须验证 `fallback_clis` 自动切换；
  2) 必须验证运行中注入链路（`run_tasks(wait=false)` + `reply_agent(message)`）；
  3) 不以手工 provider 指定作为通过条件。
- 新增针对以上链路的回归测试，避免回退到冒烟级别验证。

## Metadata
- id: err-2026-03-05-user-correction-teams-fallback-and-inject-e2e
- tags: [user-correction, teams, fallback, inject, e2e]
- links:
  - docs/error-experience/summary/entries/2026-03-05-user-correction-teams-fallback-and-inject-e2e.md
