# 2026-03-03 · 中途范围纠正未立即并入执行路径

## Context
- 用户在进行中任务里追加了明确约束：检查“今天日期”注入，并要求通过独立 chunk 注入（而非 system prompt）。

## Symptom
- 执行流仍沿原子任务推进，未在第一时间把新增约束并入当前实现与验证路径。

## Root Cause
- 对“中途追加约束”的优先级处理不够刚性，默认沿用既定改造路线。
- 缺少显式的“范围更新即刻重排”动作。

## Remediation
- 任何用户纠正或新增约束到达后，必须立即执行：
  - 暂停当前实现分支，先确认新增约束的代码触点与验证点。
  - 将新增约束并入同一提交范围（或明确拆分提交边界），不得延后到“下轮再做”。
  - 在最终验证清单中单列该新增约束对应的测试/断言。

## Follow-up
- 后续凡是“注入方式/位置”类要求（如 system prompt vs runtime chunk）都必须显式核对注入层级，并在测试里锁定。

## Metadata
- id: err-2026-03-03-midtask-scope-correction-date-chunk
- tags: [scope-correction, execution-discipline, prompt-layering, testing]
- links:
  - docs/error-experience/summary/entries/2026-03-03-midtask-scope-correction-date-chunk.md
