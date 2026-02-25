# 2026-02-11 Skills Home Runtime Compatibility

## Objective
- 让 `~/.alex/skills/<skill>/run.py` 直接执行时也能稳定加载依赖与 `.env` API keys。

## Plan
1. Baseline
- [completed] 复现 home 路径执行失败点（import / dotenv）。

2. Runtime compatibility
- [completed] 让 home skills 具备所需 Python helper 包（最小必要同步策略）。
- [completed] 增强 dotenv 搜索策略（`__file__` 失败时回退到 `ALEX_REPO_ROOT` / `cwd`），并避免误读 `~/.env`。

3. Tests
- [completed] 补充/更新单测覆盖新回退逻辑和 home 同步行为。

4. Validation
- [completed] 运行 vet/check-arch/test 全量校验；`fmt` 被仓库现存无关 lint 阻塞（`internal/app/context/manager_prompt.go:708 summarizeMap unused`）。

5. Review & delivery
- [in_progress] 结构化 code review，修复发现后再次验证并提交。

## Progress
- 2026-02-11 15:10: 完成 `EnsureHomeSkills` 的 home support scripts 同步（`~/.alex/scripts/{skill_runner,cli}`），引入独立 marker（`.repo_support_scripts_version`）。
- 2026-02-11 15:10: 完成 `load_repo_dotenv` 多源回退：`start_path` → `ALEX_REPO_ROOT` → `cwd`。
- 2026-02-11 15:11: 完成“长期兼容”修复：`~/.alex/skills` 上行搜索止步 `.alex`，避免误命中 `~/.env`。
- 2026-02-11 15:12: 定向测试通过：`go test ./internal/infra/skills -run 'SupportScripts|Backfill|ResolveSkillsRootDefaultsToHomeAndCopiesMissing|TestLoadSkipsMalformedSkillAndKeepsValidSkills'`。
- 2026-02-11 15:12: Python 测试通过：`python3 -m pytest scripts/skill_runner/tests/test_env.py -q`（8 passed）。
- 2026-02-11 15:14: 全量校验：`make vet`/`make check-arch`/`make test` 通过；`make fmt` 因仓库现存无关 lint 问题失败。
