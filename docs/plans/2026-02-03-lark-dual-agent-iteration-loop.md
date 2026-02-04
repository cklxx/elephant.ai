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
- 日志按 worktree 隔离：主/副进程启动时都会设置 `ALEX_LOG_DIR`，因此内部 `alex-service/llm/latency` 日志落在各自 worktree 的 `logs/` 下。

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

### 统一入口（推荐）

只记两个命令：

- `./lark.sh ma ...`：主 agent（main worktree 的 server + 本地 auth DB）
- `./lark.sh ta ...`：副 agent（test worktree 的 server + 自愈 loop watcher；会自动确保 `.worktrees/test` + 同步 `.env`）

用法：

```bash
./lark.sh ma start
./lark.sh ta start
```

对应关系（想看细节时再用）：
- `./lark.sh ma <cmd>` → `scripts/lark/main.sh <cmd>`
- `./lark.sh ta <cmd>` → `scripts/lark/test.sh <cmd>` + `scripts/lark/loop-agent.sh <cmd>`（启动前会 `worktree.sh ensure`）

### Worktree 管理

- `scripts/lark/worktree.sh ensure`
  - 确保 `.worktrees/test` worktree 常驻（分支 `test`）
  - 强制同步 `.env` -> `.worktrees/test/.env`

### 主/副进程管理

- `scripts/lark/main.sh start|stop|restart|status|logs|build`
  - 在主 worktree 启动/重启 `alex-server`（并确保本地 auth DB 已启动）
  - 通过 `http://127.0.0.1:${MAIN_PORT:-8080}/health` 做健康检查

- `scripts/lark/loop-agent.sh start|stop|restart|status|logs`
  - 启动/管理副 agent 的 loop（`scripts/lark/loop.sh watch`）

- `scripts/lark/test.sh start|stop|restart|status|logs|build`
  - 在 `.worktrees/test` 启动/重启 `alex-server`（并确保本地 auth DB 已启动 + `.env` 已同步）
  - 默认读取 `~/.alex/test.yaml`（也可指定绝对路径，如 `/Users/bytedance/.alex/test.yaml`）
  - 内部日志目录：默认写到 `.worktrees/test/logs`（可通过 `ALEX_LOG_DIR` 覆盖）
  - `alex-server` **总是以当前 test worktree 的代码**运行（即 `git -C .worktrees/test rev-parse HEAD`）
    - test 分支如何对齐到 `main@SHA`：由 loop 负责（`scripts/lark/loop.sh` 在每轮 cycle 开始会 `reset --hard base_sha`）
  - `FORCE_REBUILD=1` 默认开启：每次 start/restart 都会重新编译（避免“看起来没编译”的误判）

### 副 agent 自愈闭环

- `scripts/lark/loop.sh watch`
  - 每 `SLEEP_SECONDS`（默认 10s）轮询 `main` 的 SHA
  - 若发现新 SHA 且无锁，则触发一轮 `run_cycle(base_sha)`
  - 若某次循环 exhaust（修不动/门禁持续失败），会记录 `last_sha`，避免对同一个 SHA 紧密重跑（等 main 前进再跑）
  - 每轮 cycle 会重启 test bot 两次：
    1) `git reset --hard base_sha` 之后（让 test bot 跑在要验证的快照上）
    2) fast+slow gate 全部通过之后（让 test bot 跑在最终 candidate commit 上）

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
2) 安装 web 依赖（slow gate 会跑 web lint；main/test worktree 都需要）
```bash
cd web && pnpm install
cd -
./scripts/lark/worktree.sh ensure
cd .worktrees/test/web && pnpm install
cd -
```
3) 启动主/副 agent：
```bash
./lark.sh ma start
./lark.sh ta start
```

4) 让 test bot 的工作目录指向 test worktree（避免修复落在 main）：

```yaml
channels:
  lark:
    workspace_dir: /Users/bytedance/code/elephant.ai/.worktrees/test
```

---

## Logs / Artifacts

- main worktree（repo root）：
  - `logs/lark-main.log`：主 agent server stdout/stderr（nohup 重定向）
  - `logs/alex-service.log`：主 agent 内部服务日志（含 Lark gateway、任务执行）
  - `logs/alex-llm.log`：主 agent LLM 调用日志
  - `logs/alex-latency.log`：主 agent latency 日志
- test worktree（`.worktrees/test`）：
  - `logs/lark-test.log`：test server stdout/stderr（nohup 重定向）
  - `logs/alex-service.log`：test server 内部服务日志（含 `Lark message received ...`）
  - `logs/alex-llm.log`：test server LLM 调用日志
  - `logs/alex-latency.log`：test server latency 日志
  - `logs/lark-loop-agent.log`：loop watcher 进程日志（watch 输出）
  - `logs/lark-loop.log`：自愈循环总日志
  - `logs/lark-loop.fail.txt`：失败摘要（tail 200）
  - `logs/lark-scenarios.json` / `logs/lark-scenarios.md`：场景评测报告

---

## Note: main agent “莫名其妙没了” 的常见原因（已规避）

loop 在合入 `main` 后会尝试重启主 agent 以加载新代码。为了避免误伤 `dev.sh` 启动的服务：

- loop **只会**在检测到 `${MAIN_ROOT}/.pids/lark-main.pid` 存在时才重启主 agent
- 推荐主 agent 统一使用 `./lark.sh ma start` 启动（而不是 `./dev.sh`），避免端口占用/进程接管的歧义

---

## Note: 手动场景默认跳过

`alex lark scenario run` 默认 **跳过** `tags: ["manual"]` 的场景（除非显式 `--tag manual`），
避免 loop 被“专门留给自愈的故障场景”干扰。

---

## Status / Next Steps

- [x] 常驻 worktree：`.worktrees/test` + `.env` 自动同步
- [x] 主进程管理：`scripts/lark/main.sh`
- [x] test worktree server：`scripts/lark/test.sh`
- [x] 副循环进程：`scripts/lark/loop-agent.sh` + `scripts/lark/loop.sh`
- [x] CLI 场景评测：`alex lark scenario run`
- [ ] （可选）将 loop 的进度/失败摘要推送回 Lark（目前写本地 logs）
