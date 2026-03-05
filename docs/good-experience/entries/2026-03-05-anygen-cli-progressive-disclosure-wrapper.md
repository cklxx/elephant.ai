# 2026-03-05 - AnyGen skills 仓库封装为统一 CLI + 渐进式披露

## Context
- 目标：把上游 `AnyGenIO/anygen-skills` 以 elephant.ai 可执行的 `CLI + SKILL.md` 形式落地。
- 约束：保持现有 skill 兼容，不做破坏性迁移；help 流必须支持渐进式披露。

## What Worked
- 采用两层封装，职责清晰：
  - `scripts/cli/anygen/anygen_cli.py` 承载命令语义与执行逻辑（`help/task`）。
  - `skills/anygen/run.py` 只做 JSON 输入输出和路由，保持 skill 薄封装。
- 渐进式披露结构固定为 `overview -> modules -> module -> action`，让 agent 能先发现能力再调用动作。
- 在不动旧入口（`skills/anygen-task-creator`）前提下新增标准入口，规避兼容性回归。
- 先补单测再跑全量门禁，确保新逻辑正确并与仓库主流程兼容。

## Evidence
- Added:
  - `scripts/cli/anygen/anygen_cli.py`
  - `scripts/skill_runner/anygen_cli.py`
  - `skills/anygen/SKILL.md`
  - `skills/anygen/run.py`
  - tests under `scripts/cli/anygen/tests/`, `scripts/skill_runner/tests/`, `skills/anygen/tests/`
- Verification:
  - `pytest -q scripts/cli/anygen/tests/test_anygen_cli.py scripts/skill_runner/tests/test_anygen_cli.py skills/anygen/tests/test_anygen_skill.py`
  - `make dev-lint`
  - `make dev-test`

## Reusable Rule
- 对外部技能仓库的接入优先使用“统一 CLI runtime + 薄 skill wrapper”模式：
  1) CLI runtime 管命令语义与执行；
  2) skill wrapper 管输入输出与路由；
  3) help 用渐进式层级，避免一次性暴露过量细节。

## Metadata
- id: good-2026-03-05-anygen-cli-progressive-disclosure-wrapper
- tags: [skills, anygen, cli, progressive-disclosure, wrapper]
- links:
  - scripts/cli/anygen/anygen_cli.py
  - skills/anygen/SKILL.md
  - docs/plans/2026-03-05-anygen-skills-cli-wrapper.md
