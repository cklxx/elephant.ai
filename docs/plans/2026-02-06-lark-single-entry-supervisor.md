# Plan: Lark 单入口脚本 + Supervisor 守护自治

Date: 2026-02-06  
Status: In Progress  
Author: cklxx

## Summary

把本地自治迭代流程统一收敛到 `./lark.sh` 一个用户入口，内部由 `scripts/lark/supervisor.sh` 托管 `main/test/loop` 三个进程，并输出结构化状态供聊天侧可观测。

## Scope

- `lark.sh` 新命令面：`up|down|restart|status|logs|doctor|cycle --base-sha`
- 新增 supervisor 守护状态机（健康检查、自动拉起、退避、熔断、单实例锁）
- loop 输出结构化状态（`lark-loop.state.json`）
- 保留 `ma/ta` 一个周期兼容，并提示弃用
- 文档/配置说明同步
- 脚本 smoke 覆盖

## Implementation Checklist

- [x] 新增 `scripts/lark/supervisor.sh`，实现守护编排和状态写入
- [x] 将 `lark.sh` 改为唯一入口命令集
- [x] 将 `ma/ta` 改为弃用别名并转发新语义
- [x] 恢复并接入 `scripts/lark/worktree.sh sync-env`
- [x] 在 `scripts/lark/loop.sh` 输出 `lark-loop.state.json`
- [x] 新增 `tests/scripts/lark-supervisor-smoke.sh`
- [x] 更新 `docs/plans/2026-02-03-lark-dual-agent-iteration-loop.md`
- [x] 更新 `docs/reference/CONFIG.md`（Lark 本地守护相关 env）
- [x] 运行 `tests/scripts/lark-build-fingerprint.sh`
- [x] 运行 `tests/scripts/lark-supervisor-smoke.sh`
- [x] 运行 `./dev.sh lint`
- [x] 运行 `./dev.sh test`
- [ ] 分批提交并 `ff-only` 合并回 `main`
- [ ] 清理临时 worktree

## Progress Log

- 2026-02-06 11:00：完成 supervisor 脚本、单入口 `lark.sh`、loop 状态输出和 worktree env 同步修复。
- 2026-02-06 11:20：补充 `ma/ta` 弃用转发到新语义（不再暴露旧 build/脚本控制面）。
- 2026-02-06 11:25：新增 supervisor smoke 测试脚本，覆盖 run-once/start-idempotent/stop-cleanup。
- 2026-02-06 11:35：完成 `lark-build-fingerprint`、`lark-supervisor-smoke`、`./dev.sh lint`、`./dev.sh test` 全量验证通过。
