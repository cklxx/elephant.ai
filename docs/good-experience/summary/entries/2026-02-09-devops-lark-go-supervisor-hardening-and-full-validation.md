Summary: 完成 Go DevOps/Lark 进程链路硬化，修复 watcher race、PID 身份误判、supervisor 阈值与 backoff 调度问题，并通过全量 lint/test + lark smoke 验收。
Impact: 将本地链路从“可运行但边界不稳”提升到“可验证且可恢复”的工程状态，后续迭代风险显著下降。
