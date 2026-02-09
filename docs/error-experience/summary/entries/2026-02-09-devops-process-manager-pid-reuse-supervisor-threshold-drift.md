Summary: Go DevOps 进程链路存在 PID 复用误判、同名进程 wait 清理竞态、supervisor 阈值边界不一致和同步 backoff 阻塞，导致状态与恢复行为不稳定。
Remediation: 引入 PID 元数据强校验并统一 `Stop/Recover/IsRunning`，修复 wait 清理条件，阈值语义统一为 `>=` 触发 cooldown，并将 backoff 改为异步执行。
