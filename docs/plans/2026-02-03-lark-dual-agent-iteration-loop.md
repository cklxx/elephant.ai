# 本地双 Worktree 双进程自我迭代闭环（main/test 常驻）

Date: 2026-02-03  
Status: Implemented (scripts + CLI)  
Author: cklxx

---

## Summary

这是一个**本地自洽**的自我迭代系统（不依赖远端 CI/队列）：

- 主 agent 常驻在 `main` worktree（repo root），负责持续接单/编码/commit（本地）。
- 副 agent 常驻在 `test` worktree（`.worktrees/test`），负责：
  - 拉齐 `main@SHA` 基线
  - 自动跑测试（快/慢两层门禁）
  - 失败则用 codex/claude 自动修复并 commit 到 `test`
  - 通过后 **ff-only 合入 `main`**
  - 触发主 agent 重启/更新部署
- `.env` 是本地事实来源：worktree 创建/每轮测试前自动复制到 `.worktrees/test/.env`

这套设计允许主 agent “乱搞”，但副 agent 的闭环必须以**明确的 `main@SHA`**为测试对象，否则测试结果没有语义。

---

## Key Constraints（辩证逻辑）

### 1) 允许并发改 main，但测试必须绑定 base_sha

副 agent 每轮以 `base_sha = git rev-parse main` 为基线，对 `test` 分支执行：

1. `git reset --hard $base_sha`
2. 跑门禁（失败则修复并 commit）

这样每轮的“通过”都有语义：**对某个 `main@SHA`，`test` 的修复补丁使其通过**。

### 2) Git worktree 硬约束：不能在 test worktree 里更新 main 分支

`main` 分支 checkout 在主 worktree 时，Git 不允许在另一个 worktree 更新 `refs/heads/main`。

因此“合入 main”必须在主 worktree 上下文执行（本质是 `git -C <main_root> merge ...`）。

---

## Implemented Interfaces

### Worktree 管理

- `scripts/lark/worktree.sh ensure`
  - 确保 `.worktrees/test` worktree 常驻（分支 `test`）
  - 强制同步 `.env` -> `.worktrees/test/.env`

### 主/副进程管理

- `scripts/lark/main.sh start|stop|restart|status|logs|build`
  - 在主 worktree 启动/重启 `alex-server`
  - 通过 `http://127.0.0.1:${MAIN_PORT:-8080}/health` 做健康检查

- `scripts/lark/test.sh start|stop|restart|status|logs`
  - 启动/管理副 agent 的 loop（`scripts/lark/loop.sh watch`）

### 副 agent 自愈闭环

- `scripts/lark/loop.sh watch`
  - 每 `SLEEP_SECONDS`（默认 2s）轮询 `main` 的 SHA
  - 若发现新 SHA 且无锁，则触发一轮 `run_cycle(base_sha)`

- `scripts/lark/loop.sh run --base-sha <sha>`
  - 手动触发一次闭环（便于 debug）

并发互斥：
- 使用 `tmp/lark-loop.lock/`（`mkdir` 原子锁）避免重复循环

### CLI hack：Lark 体验评测（in-process）

新增 CLI 命令（无需启动 container）：

```bash
alex lark scenario run \
  --dir tests/scenarios/lark \
  --json-out logs/lark-scenarios.json \
  --md-out logs/lark-scenarios.md
```

特性：
- 基于 `tests/scenarios/lark/*.yaml`，通过 `Gateway.InjectMessage()` in-process 注入消息并断言 outbound calls。
- 输出 JSON/Markdown 报告，退出码语义化：
  - `0`：全部通过
  - `1`：存在失败场景
  - `2`：参数/加载/解析错误

---

## Gate Policy（已实现）

### Fast gate（每轮自愈循环都跑）
1) `go run ./cmd/alex lark scenario run ...`  
2) `CGO_ENABLED=0 go test ./... -count=1`

### Slow gate（合入 main 前跑一次）
1) `./dev.sh lint`  
2) `./dev.sh test`

---

## How To Run

1) 确保 `.env` 已准备好（参考 `.env.example`）
2) 启动主 agent：
```bash
./scripts/lark/main.sh start
```
3) 启动副 agent loop：
```bash
./scripts/lark/test.sh start
```

---

## Logs / Artifacts

- `logs/lark-loop.log`：自愈循环总日志
- `logs/lark-loop.fail.txt`：失败摘要（tail 200）
- `logs/lark-scenarios.json` / `logs/lark-scenarios.md`：场景评测报告

---

## Status / Next Steps

- [x] 常驻 worktree：`.worktrees/test` + `.env` 自动同步
- [x] 主进程管理：`scripts/lark/main.sh`
- [x] 副循环进程：`scripts/lark/test.sh` + `scripts/lark/loop.sh`
- [x] CLI 场景评测：`alex lark scenario run`
- [ ] （可选）将 loop 的进度/失败摘要推送回 Lark（目前写本地 logs）

