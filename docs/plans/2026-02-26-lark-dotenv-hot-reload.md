# 2026-02-26 Lark .env Hot Reload Fix

## Goal
修复 Lark 运行时改 `.env` 后不自动生效的问题，使无需重启即可更新由 `.env` 提供的配置值（如 `TAVILY_API_KEY`、Lark 凭证引用值等）。

## Findings
- 当前 runtime watcher 仅监听 `config.yaml`（含默认路径），不监听 `.env`。
- watcher 触发 reload 时只刷新 managed overrides，不会重新加载 `.env`。
- `LoadDotEnv()` 只在变量不存在时写入进程环境，因此即使再次调用也不会覆盖旧值。

## Plan
- [x] 在 config 包新增“受管 `.env` 变量重载”能力：
  - 仅更新最初由 `.env` 注入的键
  - 不覆盖外部显式注入的环境变量
  - 支持删除 `.env` 中已移除的受管键
- [x] 提供 `.env` 默认 watch path 解析（支持 `ALEX_DOTENV_PATH` 与默认 `.env`）。
- [x] 在 bootstrap watcher 中加入 `.env` 监听，并在 reload 前执行受管 `.env` 重载。
- [x] 补充单测覆盖值更新、键删除、外部 env 保护。
- [x] 运行相关测试并记录结果。

## Validation
- `go test ./internal/shared/config ./internal/delivery/server/bootstrap -count=1` ✅
