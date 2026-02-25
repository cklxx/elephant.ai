# Plan: Codex / Claude Code 调用梳理 (2026-01-31)

## Goal
- 梳理仓库内如何调用 Codex 与 Claude Code（配置、入口、调用路径与关键注意事项）。

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Scope
1. 扫描仓库中 codex/claude code 的集成点与调用入口。
2. 补充配置与环境变量要求（YAML 示例）。
3. 引用官方文档（OpenAI/Codex、Anthropic/Claude Code）作为权威补充。
4. 形成可直接执行的步骤与注意事项。

## Progress
- 2026-01-31: Plan created; engineering practices reviewed.
- 2026-01-31: Added external agent call flow + tables in `docs/reference/external-agents-codex-claude-code.md`.
- 2026-01-31: Ran `./dev.sh lint` and `./dev.sh test` (LC_DYSYMTAB linker warnings observed).
- 2026-01-31: Linked official Codex/Claude Code docs and clarified CLI requirements.
