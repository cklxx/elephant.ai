# 2026-03-05 · 用户纠正：参数传递禁止使用 JSON 形式

## Context
- 在 skill 调用提示中使用了 `run.py '{...}'` 形式。
- 用户明确要求：`禁止用json 传参数 记住`。

## Symptom
- 调用约定表达会引导 JSON 参数传递，与用户偏好冲突。

## Root Cause
- 沿用了历史 skill 调用样式，未对齐当前任务中的参数约束。

## Remediation
- 当前任务及后续同类改动中，参数示例统一采用 CLI flags/positional 参数，不使用 JSON 参数块。
- 文案层禁止出现 `'{...}'` 形式的参数示例。
- 若脚本确需结构化输入，优先通过文件路径或多 flag 方式表达。

## Metadata
- id: err-2026-03-05-user-correction-no-json-params
- tags: [user-correction, cli, interface-contract]
- links:
  - docs/error-experience/summary/entries/2026-03-05-user-correction-no-json-params.md
