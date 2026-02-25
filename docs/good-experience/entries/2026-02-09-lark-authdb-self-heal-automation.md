# 2026-02-09 — Lark/AuthDB 自愈自动化落地

## Practice
- 在 Lark 进程管理与 auth DB 初始化链路引入“自动清理孤儿进程 + 可重试恢复”。

## Why It Worked
- 把故障从“事后人工排查”前移到“启动前自动治理”：main/test 每次启动前主动清理未受管 `alex-server lark`。
- auth DB 初始化对 `too many clients already` 具备针对性恢复动作（清理 + 退避重试），避免一次性失败直接打断流程。
- 保留 setup 日志上下文（append），便于快速确认真实失败点。

## Outcome
- 本地环境可自动回收大量残留 test 进程，显著降低 DB 连接耗尽概率。
- `scripts/setup_local_auth_db.sh` 在相同环境下恢复为可稳定成功执行。
- Lark supervisor 相关脚本 smoke 测试保持通过。
