# Plan: 双配置文件监控异步热更新 + lark.sh 永远重启

## Status: In Progress
## Date: 2026-02-04

## Problem
- 运行中的 alex-server 需要同时监控两个 config 文件：`~/.alex/config.yaml` 与 `~/.alex/test.yaml`（以及 `ALEX_CONFIG_PATH` 指向的生效配置），任一变更触发异步 reload（debounce），且不阻塞任何执行。
- `./lark.sh ma|ta` 需要做到每次执行都强制重启（`start` 等价 `restart`，包含 loop-agent）。

## Plan
1. 新增 helper：生成默认 watch 路径列表（去重、稳定顺序）并补单测。
2. bootstrap 层按路径列表启动多个 watcher，统一 beforeReload（RefreshOverrides）+ reload（RuntimeConfigCache）。
3. 调整 `lark.sh`：`ma/ta` 下 `start` 默认映射为 `restart`，并更新 usage 文案。
4. 运行 full lint + tests。

## Progress
- [x] Add `DefaultRuntimeConfigWatchPaths` helper + tests.
- [x] Watch both config files in server bootstrap.
- [x] Force lark.sh ma/ta to always restart.
- [ ] Run `./dev.sh lint` and `./dev.sh test`.
