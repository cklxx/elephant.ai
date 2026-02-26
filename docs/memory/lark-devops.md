# Lark & DevOps — Long-Term Memory Topic

Updated: 2026-02-26 15:00

Extracted from `long-term.md` to keep the main file concise.

---

## Lark Local Operations

- Lark 本地链路如果 PID 文件写到包装 shell 而非真实 `alex-server`，会造成孤儿进程累积并耗尽 auth DB 连接；后台启动必须保证记录真实子进程 PID。
- auth DB 本地初始化遇到 `too many clients already` 应执行"孤儿 Lark 进程清理 + 退避重试"，比一次失败直接降级更稳定。
- Lark loop gate 的 codex auto-fix 应默认关闭并显式开关启用（`LARK_LOOP_AUTOFIX_ENABLED=1`），否则会出现"非预期自动改代码"体验。
- Lark 普通会话链路里的 background progress listener 清理必须使用 `Release()`（不是 `Close()`），否则 foreground 返回后会丢失 coding task 完成通知。
- Lark `/model use --chat` should resolve at chat scope first (`channel+chat_id`) with legacy `chat+user` compatibility fallback, otherwise group chats can miss pinned credentials.
- Lark callbacks: `channels.lark` supports `${ENV}` expansion; callback token/encrypt key also have env fallback keys in bootstrap to avoid silent callback disablement.

## DevOps & Process Management

- DevOps `ProcessManager` 对磁盘 PID 恢复/停止不能只做 `kill(0)`；必须持久化并校验进程身份（命令行签名），避免 PID 复用误判和误杀。
- 同名进程快速替换时，旧进程 `Wait` 回调清理必须确认 map 里仍是同一实例，防止误删新进程追踪状态。
- Supervisor 重启阈值语义应统一为"达到上限触发 cooldown（>=）"，且 backoff 要异步执行，避免阻塞同一 tick 的其他组件健康处理。

## Auth & Infrastructure

- To reduce auth session-loss regressions under multi-process dev load, cap auth DB pool connections with `auth.database_pool_max_conns` / `AUTH_DATABASE_POOL_MAX_CONNS` (default `4`).
- Server auth config should support env fallback for JWT/OAuth fields; in development-like environments auth DB failures should degrade to memory stores instead of disabling the whole auth module.
