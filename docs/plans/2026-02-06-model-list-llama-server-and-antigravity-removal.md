# Plan: model list 移除 antigravity_ide，新增 llama_server 自动列出

**Date:** 2026-02-06  
**Status:** done  
**Owner:** cklxx + Codex

## Goal
- `./alex model list` 不再显示 `antigravity_ide`/antigravity 订阅来源。
- 增加 `llama_server` 支持：当本地 llama.cpp server 可用时，自动列出可用模型。

## Non-goals
- 不删除 antigravity LLM provider 的底层实现（仅调整 model list / selection 命令行为）。
- 不改动运行时主请求链路（仅模型目录与命令侧展示/选择）。

## Approach
1. 在 subscription catalog 层移除 antigravity provider 的列出逻辑。
2. 在 catalog 层新增 llama_server target resolver 与在线探测，仅服务可用时追加 provider。
3. CLI model 命令注入 llama_server resolver，并支持 `llama_server` 作为 `model use` 别名。
4. 更新测试覆盖（TDD）与文案示例，执行 lint/test 回归。

## Milestones
- [x] 更新/新增测试（先失败）
- [x] 实现 antigravity 列表移除
- [x] 实现 llama_server 在线列出
- [x] 更新 CLI 文案与 provider alias
- [x] lint + test 回归

## Progress Log
- 2026-02-06 15: 完成实现链路定位（`cmd/alex/cli_model.go` + `internal/app/subscription/catalog.go` + 对应测试）。
- 2026-02-06 15: 按新要求将对外 provider 统一为 `llama_server`，移除 `llama.cpp` 暴露与 antigravity 列表输出。
- 2026-02-06 16: 将 `llama_server` 默认 host 统一调整为 `http://127.0.0.1:8082`（含 `/v1`）。
- 2026-02-06 16: 完成 `./dev.sh lint` 与 `./dev.sh test` 全量回归通过。
