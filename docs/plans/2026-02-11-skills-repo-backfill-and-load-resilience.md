# 2026-02-11 Skills Repo Backfill And Load Resilience

## Objective
- 以仓库 `skills/` 为基准做一次性补齐（含同名覆盖）到 `~/.alex/skills`。
- 修复单个 skill 加载失败导致全量 skills 不可用的问题。

## Plan
1. Baseline & diff
- [completed] 复核 repo/home skills 差异与当前加载优先级行为。

2. One-time backfill (repo authoritative)
- [completed] 在默认 `~/.alex/skills` 路径下引入“仅一次”的仓库覆盖补齐机制，并写入版本 marker，避免每次覆盖。
- [completed] 补充 discovery 相关测试，覆盖一次性覆盖、重复运行不覆盖、marker 生效。

3. Partial-failure tolerance
- [completed] 调整 skills 加载逻辑：单个 skill 解析失败时跳过该 skill 并继续加载其余 skills。
- [completed] 补充测试，覆盖 malformed skill 不影响其它合法 skill。

4. Validation
- [completed] 运行格式化、相关单测与全量 lint/test（按仓库要求）。

5. Review & delivery
- [completed] 按 code-review skill 做结构化审查，修复问题后再次验证。
- [completed] 以增量 commit 提交并回合并到 `main`，清理临时 worktree。
