Summary: `scripts/lark/test.sh` 将 PID 记录到包装 shell 而非真实 `alex-server`，导致孤儿 Lark 进程累积并耗尽 auth Postgres 连接（`too many clients already`），触发 supervisor 持续降级。
Remediation: 修正 PID 捕获、加入孤儿进程自动清理脚本并在 main/test 启动链路接入；auth DB setup 对连接耗尽错误增加自动清理与重试。
