# 2026-03-03 · 用户缩小范围后未立即切换执行边界

## Context
- 用户最初要求测试 Lark 日程和任务。
- 用户随后明确纠正：只测任务，日程无意义。

## Symptom
- 执行节奏仍残留原始双域目标，存在继续扩展到日程域的风险。

## Root Cause
- 缺少“收到范围纠偏后立即冻结旧目标”的强制步骤。
- 未把用户最新约束转写成可核验的执行清单。

## Remediation
- 收到用户纠偏后，立即执行三步：
  - 用一句话复述新范围（例如：仅任务，不含日程）。
  - 停止旧范围下的命令与验证。
  - 在计划/检查清单里写明排除项，避免回流。
- 后续汇报中显式标注“已按最新范围执行”。

## Follow-up
- 将该规则作为默认动作，适用于任何中途范围变更。

## Metadata
- id: err-2026-03-03-user-scope-correction-task-only
- tags: [scope-control, user-correction, execution-discipline]
- links:
  - docs/error-experience/summary/entries/2026-03-03-user-scope-correction-task-only.md
  - docs/plans/2026-03-03-lark-task-list-oauth-fix.md
