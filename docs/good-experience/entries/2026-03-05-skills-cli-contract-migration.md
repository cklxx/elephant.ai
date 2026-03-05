# 2026-03-05 - Skills JSON 契约全量迁移到 CLI 契约

## Context
- 目标：将 `skills/*/run.py` 的 JSON 顶层入参/JSON stdout 契约统一迁移为 CLI 参数 + 文本 stdout/stderr。
- 范围：runtime 入口、skills 暴露元数据、smoke/e2e tooling、SKILL 文档与 catalog。
- 约束：一次性切换，不保留 JSON 顶层 payload 兼容。

## What Worked
- 先做全量扫描再分三条 ownership 并行执行（runtime / infra+tooling / docs），能在大规模机械迁移时保持低冲突。
- 抽出共享 `scripts/skill_runner/cli_contract.py`，避免 25 个 skill 入口重复实现解析/输出逻辑。
- 在 infra 层统一 `ExecCommand()` 并从 `SourcePath` 推导 skill 目录，修复 `skill.Name` 与目录名不一致的路径风险。
- 同步更新 e2e case generator 与 dataset，避免“运行时已迁移、评估链路仍旧 JSON”造成的持续回归。

## Evidence
- Runtime:
  - `scripts/skill_runner/cli_contract.py`
  - `skills/*/run.py`
- Infra/tooling:
  - `internal/infra/skills/*.go`
  - `internal/infra/tools/builtin/session/skills.go`
  - `scripts/skill_runner/{smoke_all_skills.py,generate_agent_e2e_cases.py}`
  - `evaluation/skills_e2e/cases.yaml`
- Docs:
  - `skills/*/SKILL.md`
  - `AGENTS.md`
  - `docs/guides/{code-review-guide.md,engineering-workflow.md}`
  - `web/lib/generated/skillsCatalog.json`
- Verification:
  - `alex dev lint`
  - `alex dev test`
  - `pytest -q skills`
  - `PYTHONPATH=scripts pytest -q scripts/skill_runner/tests`
  - `go test ./internal/infra/skills ./internal/infra/tools/builtin/session`

## Reusable Rule
- Skills 大规模协议迁移优先采用：
  1) 契约抽象先收敛（共享 parser/renderer）；
  2) 运行时/元数据/评估链路同批次切换；
  3) 文档与生成产物最后一次性同步，避免新旧契约混用。

## Metadata
- id: good-2026-03-05-skills-cli-contract-migration
- tags: [skills, cli, migration, subagent, tooling]
- links:
  - scripts/skill_runner/cli_contract.py
  - docs/plans/2026-03-05-skills-cli-io-migration.md
  - evaluation/skills_e2e/cases.yaml
