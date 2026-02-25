# Plan: Lark Supervisor Cooldown Autofix

Date: 2026-02-06  
Status: In Progress  
Author: cklxx

## Summary

在 `lark.sh -> supervisor -> loop` 体系中新增自动故障修复执行器：  
Supervisor 仅在进入 cooldown 熔断时触发 `autofix.sh`，由 Codex 自动修复并做最小验证，通过后回流到 `main` 并触发组件重启。

## Scope

- 新增 `scripts/lark/autofix.sh`
- 在 `scripts/lark/supervisor.sh` 增加 autofix 触发/状态/限流/去重逻辑
- 扩展 `lark-supervisor.status.json` autofix 字段
- 新增 `tests/scripts/lark-autofix-smoke.sh`
- 更新 `tests/scripts/lark-supervisor-smoke.sh`
- 更新配置文档

## Implementation Checklist

- [x] 新增 `scripts/lark/autofix.sh`（隔离 worktree、Codex 修复、验证、rebase+ff merge）
- [x] Supervisor 增加 autofix 环境变量、状态字段、cooldown 触发逻辑
- [x] Supervisor 状态输出扩展 autofix 字段
- [x] 新增 `tests/scripts/lark-autofix-smoke.sh`
- [x] 更新 `tests/scripts/lark-supervisor-smoke.sh`
- [x] 更新 `docs/reference/CONFIG.md`
- [x] 脚本可执行位与语法校验
- [x] 运行 `tests/scripts/lark-build-fingerprint.sh`
- [x] 运行 `tests/scripts/lark-supervisor-smoke.sh`
- [x] 运行 `tests/scripts/lark-autofix-smoke.sh`
- [x] 运行 `./dev.sh lint`
- [x] 运行 `./dev.sh test`
- [ ] 分批提交并 ff-only 合并回 main，清理 worktree

## Progress Log

- 2026-02-06 12:35: 创建新 worktree 分支 `eli/lark-supervisor-autofix` 并同步 `.env`。
- 2026-02-06 12:45: 完成 `autofix.sh` 首版（触发、状态写入、Codex 执行、最小验证、rebase/merge）。
- 2026-02-06 12:55: 完成 supervisor 侧 autofix 状态机集成（cooldown 触发、去重、限流、状态字段、成功重启）。
- 2026-02-06 13:10: 完成 `lark-supervisor-smoke` 与新增 `lark-autofix-smoke`，并通过 `./dev.sh lint`、`./dev.sh test`。
