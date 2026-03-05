# 2026-03-05 · 用户纠正：team 能力与产物应为一级 CLI 语义，不应挂在 Lark 子命令下

## Context
- 任务目标：修复 team CLI 命令入口与 skills 触发可发现性。
- 初始路径：沿用了 `alex lark team status` 这一历史层级。
- 用户纠正：`用独立的cli和产物吧`，明确要求 team 能力与产物语义独立于 channel 子命名。

## Symptom
- team 运行态查询入口被绑定在 `lark` 子命令下，语义上被误解为 channel 专属能力。
- skills catalog 的紧凑提示未显式暴露 CLI runner 信号，LLM 难以直接判断应通过 `shell_exec` 调用 `run.py`。

## Root Cause
- 将历史命令分组（按渠道）惯性沿用到“跨渠道的运行时能力”。
- 为压缩 prompt 过度裁剪 skills 元数据，丢失了关键执行路径提示。

## Remediation
- 对跨渠道运行时能力（如 team runtime）采用一级 CLI 命令（`alex team ...`），不再挂在 `alex lark ...`。
- 在可注入给 LLM 的 skills 紧凑目录中保留 runner 字段（例如 `py`），并给出统一调用规则：
  - `runner=py` → `shell_exec python3 skills/<name>/run.py '{...}'`
- 将“命令层级是否体现领域边界”纳入 CLI 改动检查项：跨渠道能力不得放在单渠道子命令下。

## Follow-up
- 后续新增 CLI 能力时先判定归属：
  1) channel 专属行为 → `alex <channel> ...`
  2) runtime/engine 通用行为 → 一级命令 `alex <capability> ...`
- 任何 skills prompt 压缩改动，必须保留最小可执行信号（runner/exec 约定）。

## Metadata
- id: err-2026-03-05-user-correction-team-cli-and-artifacts-should-be-first-class
- tags: [user-correction, cli, skills, prompt-context, architecture]
- links:
  - docs/error-experience/summary/entries/2026-03-05-user-correction-team-cli-and-artifacts-should-be-first-class.md
