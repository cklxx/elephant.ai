# 2026-03-05 · 用户纠正：team-cli 只需 skill 指南，不需要 run.py 包装器

## Context
- 目标：让 LLM 会用 team CLI。
- 偏差：准备新增 `skills/team-cli/run.py` 包装层。
- 用户纠正：`这个就不用run.py`。

## Symptom
- 对已明确且稳定的 CLI 命令增加了不必要的中间包装层。

## Root Cause
- 将“可执行 skill”默认等同为“必须有 run.py”，忽略了用户要的是直接真实 CLI 调用。

## Remediation
- 对“命令已稳定且用户要求真实直连调用”的 skill，优先纯 `SKILL.md` 指南，不新增 `run.py`。
- 只有在存在参数归一化、安全拦截、协议转换等明确需求时，才增加 wrapper 脚本。

## Metadata
- id: err-2026-03-05-user-correction-team-cli-skill-no-runpy
- tags: [user-correction, skills, cli, simplification]
- links:
  - docs/error-experience/summary/entries/2026-03-05-user-correction-team-cli-skill-no-runpy.md
