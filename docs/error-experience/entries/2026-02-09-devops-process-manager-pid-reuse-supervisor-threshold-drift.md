# 2026-02-09 - Go DevOps 进程管理存在 PID 复用误判与 supervisor 阈值/调度偏差

## Error
- `ProcessManager` 在同名进程快速替换时，旧进程 `Wait` goroutine 可能误删新进程追踪。
- PID 文件只做 `kill(0)` 存活检查，存在 PID 复用下误判/误杀风险。
- `Supervisor` 重启阈值语义不一致（`<` vs `>`），且 backoff `Sleep` 阻塞整个 tick，拖慢其他组件巡检。

## Impact
- `dev up/status/restart` 在边界并发下可能出现“明明在跑却显示 down”或错误清理 PID。
- 重启风暴触发边界不可预测，排障成本高。
- 某组件进入 backoff 时，其他组件健康处理被串行阻塞。

## Root Cause
- 进程生命周期清理逻辑未校验 map 当前条目是否仍为同一进程实例。
- 缺失 PID 身份元数据与命令行比对机制。
- supervisor 代码路径对“达到阈值”与“超过阈值”的定义不一致，并采用同步睡眠。

## Remediation
- `ProcessManager` 写入 PID + 元数据（命令行签名），`Stop/IsRunning/Recover` 统一执行强身份校验；不匹配即清理陈旧 PID，不做停止操作。
- `Wait` 回调仅在 map 条目仍指向同一 `ManagedProcess` 时才删除追踪/清理文件。
- 重启阈值统一为“达到上限即进入 cooldown”（`>=`）。
- 重启 backoff 改为异步延迟执行，避免阻塞 tick 主循环。

## Status
- fixed
