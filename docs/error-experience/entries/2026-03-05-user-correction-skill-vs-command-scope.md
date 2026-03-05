# 2026-03-05 · 用户纠正：需求是新增 skill 引导 LLM 调用 team CLI，不是新增 CLI 命令

## Context
- 目标：让 LLM 能稳定使用 team CLI。
- 偏差：实现了一级 `skills` CLI 命令，超出了用户要求。
- 用户纠正：`不是增加skills 命令 是增加一个skills 让 llm 会用 team cli`。

## Symptom
- 方案偏向“人用命令扩展”，而非“LLM 可用 skill 注入”。
- 产生不必要的命令面扩张，增加维护面。

## Root Cause
- 将“独立 skills”误解为“新增 CLI 子命令”，未先对齐执行载体（LLM skill vs human CLI）。

## Remediation
- 当用户目标是“让 LLM 会做 X”，默认优先：新增/改造 skill（frontmatter 触发 + run.py/调用约定），而非新增人类 CLI 命令。
- CLI 新命令仅在用户明确要求“人类操作入口”时才新增。
- 实施前做一句自检：`这次改动是给 LLM 还是给人类用户？` 若答案是 LLM，则先看 `skills/`。

## Metadata
- id: err-2026-03-05-user-correction-skill-vs-command-scope
- tags: [user-correction, scope-control, skills, cli]
- links:
  - docs/error-experience/summary/entries/2026-03-05-user-correction-skill-vs-command-scope.md
