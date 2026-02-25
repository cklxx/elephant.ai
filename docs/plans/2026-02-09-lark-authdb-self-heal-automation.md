# 2026-02-09 — Lark/AuthDB 自愈自动化（避免 `too many clients` + supervisor 降级）

## 背景
- 现象：`./dev.sh` 提示 local auth DB setup failed；`alex dev lark` 长期 `degraded`，`test restart failed`。
- 已确认根因：大量未受管的 `.../alex-server lark` 进程占满 Postgres 连接，`setup_local_auth_db.sh` 在 migration 阶段报 `FATAL: sorry, too many clients already`，随后 supervisor 进入重启失败循环。

## 目标
- 在不依赖人工干预的情况下，自动识别并清理孤儿 Lark 进程，释放 auth DB 连接。
- 当 auth DB 暂时饱和时自动重试迁移/seed，减少一次性失败。
- 保留更清晰日志，便于快速诊断。

## 计划
- [x] 定位并复现：确认连接打满 + supervisor 失败链路
- [x] 新增进程自愈脚本：识别受管 PID，清理孤儿 `alex-server lark`
- [x] 在 `scripts/lark/main.sh`、`scripts/lark/test.sh` 接入自动清理
- [x] 在 `scripts/setup_local_auth_db.sh` 接入“too many clients”自动清理 + 重试
- [x] 修正 auth DB setup 日志覆盖问题，保留失败上下文
- [x] 在当前环境验证：孤儿清理 + auth DB setup 成功
- [x] 执行 lint 校验（`./dev.sh lint` 通过）
- [ ] 执行 test 全量校验（受已有 bootstrap race 失败阻断，非本次改动引入）
- [x] 记录 error/good experience 与 long-term memory 更新

## 进度记录
- 2026-02-09 11:00：完成根因确认，确定自动化修复路径（孤儿进程清理 + DB 重试）。
- 2026-02-09 11:25：确认 `scripts/lark/test.sh` PID 采集存在包装进程写入问题（导致真实 `alex-server` 成为孤儿）。
- 2026-02-09 11:45：完成脚本改造与 smoke/lint 验证；`./dev.sh test` 仍被既有 `runtime_watcher` race 失败阻断。
