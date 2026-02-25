# Plan: dev.sh 支持 web/lark 分离控制并提供一键全起

> Created: 2026-02-09
> Status: completed
> Trigger: 用户要求“dev.sh 启动所有，但 web 和 lark 要分离开”

## Goal & Success Criteria
- **Goal**: 让 `dev.sh` 同时支持“统一入口”和“web/lark 分离启停”，避免相互误伤。
- **Done when**:
  - `./dev.sh up` 仍只管理 backend+web（不触发 lark）。
  - 新增 `./dev.sh lark-up|lark-down|lark-status`，只管理 lark supervisor 栈。
  - 新增 `./dev.sh all-up|all-down|all-status`，统一入口但内部保持 web/lark 分离。
  - 执行 `down` 不会停掉 lark；执行 `lark-down` 不会停掉 web/backend。
- **Non-goals**:
  - 不改动 lark supervisor 状态机。
  - 不重构进程管理框架（ProcessManager）。

## Current State
- `dev.sh` 仅管理 sandbox/authdb/backend/web。
- Lark 通过 `scripts/lark/*.sh` 独立管理，当前没有挂到 `dev.sh` 统一入口。
- 用户容易在“服务全起”与“lark 独立运行”之间混淆，且状态观察分散。

## Task Breakdown

| # | Task | Files | Size | Depends On |
|---|------|-------|------|------------|
| 1 | 在 `dev.sh` 增加 lark 生命周期命令封装 | `dev.sh` | M | — |
| 2 | 增加 all-* 复合命令，串联但不互相 stop | `dev.sh` | S | T1 |
| 3 | 更新 help/usage 文案，明确边界 | `dev.sh` | S | T1 |
| 4 | 手工回归验证（down/up + lark-up/down + all-status） | `dev.sh` | S | T1,T2 |

## Technical Design
- **Approach**:
  - 在 `dev.sh` 内新增 `cmd_lark_up/cmd_lark_down/cmd_lark_status/cmd_lark_logs`，分别调用 `scripts/lark/supervisor.sh` 的对应命令。
  - 新增 `cmd_all_up/cmd_all_down/cmd_all_status`：
    - `all-up`: 先 `cmd_up` 再 `cmd_lark_up`。
    - `all-down`: 先 `cmd_lark_down` 再 `cmd_down`（可选保留 sandbox 逻辑与 `down` 一致）。
    - `all-status`: 聚合 `cmd_status` + `cmd_lark_status`。
  - 保持现有 `up/down/status` 语义不变，避免破坏既有工作流。
- **Alternatives rejected**:
  - 把 lark 进程直接纳入 `dev.sh` PID 管理：侵入性高，且与现有 lark supervisor 重复。
  - 修改默认 `up` 为自动拉起 lark：会破坏“lark 独立运行”诉求。
- **Key decisions**:
  - 采用“统一入口 + 子域命令分离”而非“统一进程模型”。

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `dev.sh` 在非 main worktree 解析 lark 路径失败 | M | M | 复用现有 root 解析逻辑并提前检查脚本可执行 |
| `all-down` 行为与用户预期不一致 | M | M | 在 usage 明确 all/down 语义差异，并保持 `down` 不动 |
| 新命令引发已有自动化脚本兼容问题 | L | M | 不改旧命令语义，仅新增命令 |

## Verification
- 运行：
  - `./dev.sh lark-status`
  - `./dev.sh lark-up`
  - `./dev.sh status`
  - `./dev.sh all-status`
  - `./dev.sh lark-down`
- 回滚：
  - 回退本次对 `dev.sh` 的改动即可恢复旧行为。

## Progress Log
- 2026-02-09 20:03：新增 `lark-up/down/status/logs` 与 `all-up/down/status` 命令，保持 `up/down/status` 语义不变。
- 2026-02-09 20:05：按用户要求将默认入口切换为 `all-up`（无参数执行）。
- 2026-02-09 20:09：完成命令级验证（`help`/`status`/`all-status`/`lark-status`）。
- 2026-02-09 20:12：执行全量 `./dev.sh lint` 通过；`./dev.sh test` 失败于仓库既有问题（`internal/delivery/channels/lark` race、`internal/shared/config` getenv guard），与本次脚本改动无直接关联。
