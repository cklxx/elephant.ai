# Plan: Lark 单进程响应隔离与配置双文件强约束

> Created: 2026-02-10
> Status: in-progress
> Trigger: cklxx 反馈 Lark 仍存在重复回复，怀疑双进程/配置混用，要求系统性修复并保证 main/test YAML 隔离；随后补充要求统一全局进程管理、PID 放公共目录、并打印每个 Lark 进程实际配置文件。

## Goal & Success Criteria
- **Goal**: 保证 Lark 运行时满足“一个 agent 一个独立进程响应”，避免 main/test 因配置/身份冲突导致重复回复。
- **Done when**:
  - main/test 使用同一配置路径时启动被硬阻断。
  - main/test 解析到同一 Lark 身份（app_id 维度）时启动被硬阻断。
  - 每个 Lark agent 进程绑定唯一身份锁，冲突进程无法并发启动。
  - Lark 全链路 PID 统一收敛到 `<config_dir>/pids`（main/test/loop/supervisor）。
  - main/test 启动与状态输出明确打印该进程使用的配置文件路径。
  - 现有 smoke + 新增测试通过，`./dev.sh lint` 与 `./dev.sh test` 通过。
- **Non-goals**:
  - 不改动 Lark 网关业务语义（消息过滤/工具调用流程）。
  - 不引入跨机分布式锁，仅保证本机 dev/supervisor 链路一致性。

## Current State
- `scripts/lark/main.sh` 与 `scripts/lark/test.sh` 分别默认读取 `~/.alex/config.yaml` 与 `~/.alex/test.yaml`，但缺少运行时硬约束来阻止配置/身份冲突。
- `scripts/lark/supervisor.sh` 会同时维护 main/test 进程健康；若两者实质绑定同一 Lark 身份，仍可能双进程重复响应。
- 当前仅有 orphan cleanup 与 PID 级管理，尚未提供“Lark 身份级互斥锁”。

## Task Breakdown

| # | Task | Files | Size | Depends On |
|---|------|-------|------|------------|
| 1 | 新增 Lark 身份解析与锁工具（配置解析、identity lock acquire/release、main/test 配置隔离断言） | `scripts/lark/identity_lock.sh` | M | — |
| 2 | main/test 启停接入身份锁，防止冲突并清理 stale lock | `scripts/lark/main.sh`, `scripts/lark/test.sh` | M | T1 |
| 3 | supervisor 启动/doctor 接入 main+test 配置双文件与身份冲突校验 | `scripts/lark/supervisor.sh` | S | T1 |
| 4 | 增加脚本级回归测试覆盖（配置冲突阻断 + identity lock 行为） | `tests/scripts/lark-identity-lock.sh`, `tests/scripts/lark-supervisor-smoke.sh` | M | T1-T3 |
| 5 | Lark PID 目录统一到公共目录（`<config_dir>/pids`）并同步 CLI/devops 路径 | `scripts/lark/*.sh`, `cmd/alex/dev_lark.go`, tests | M | T1-T4 |
| 6 | main/test 显式打印正在使用的配置文件，补全全局 orphan 清理策略 | `scripts/lark/main.sh`, `scripts/lark/test.sh`, `scripts/lark/supervisor.sh` | S | T1-T5 |
| 7 | 全量验证、代码审查、增量提交、合并 main | lint/test + review artifacts | M | T1-T6 |

## Technical Design
- **Approach**: 新增独立 shell 库解析 `channels.lark.app_id`（含简单 env 占位扩展），生成 identity（优先 app_id，回退 config 绝对路径），在 `<config_dir>/pids/lark-identities/` 下维护 identity lock 文件。main/test 启动前做可用性校验，启动后写入锁，停止时释放锁；stale 锁按 PID 存活自动清理。supervisor 在 `start/run/doctor` 入口执行 main/test 配置路径与身份冲突校验，冲突即失败，避免双进程并发响应同一身份。并将 test/loop/supervisor PID 全量迁移到 `<config_dir>/pids`，形成统一全局 PID 管理目录。
- **Alternatives rejected**:
  - 在 Lark 网关层做消息去重：无法阻止双进程资源竞争，且掩盖进程隔离问题。
  - 仅做文档约束：无法保证运行时强制执行，重复问题可再次出现。
- **Key decisions**:
  - 以“身份锁 + 启动前硬校验”作为第一防线，优先阻断冲突进程而不是事后清理。
  - 把配置隔离规则前置到 supervisor 链路，确保自治模式下始终执行。

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| YAML 轻量解析对复杂写法兼容不足 | M | M | 仅提取 `channels.lark.app_id` 标量；缺失时回退 config path identity，不阻塞合法场景 |
| 锁文件 stale 导致误阻断 | M | M | 校验 lock pid 存活；死进程锁自动清理 |
| 新约束影响既有脚本 smoke | L | M | 扩充脚本测试，覆盖正常与冲突分支 |

## Verification
- `tests/scripts/lark-identity-lock.sh`
- `tests/scripts/lark-supervisor-smoke.sh`
- `./dev.sh lint`
- `./dev.sh test`
- 回滚方案：若新锁机制导致不可恢复启动失败，可回退新增 identity lock 引入点（T2/T3）并保留测试作为复现依据。

## Progress Log
- 2026-02-10: 已完成 identity lock 引入与 main/test/supervisor 接入，并新增脚本回归测试。
- 2026-02-10: PID 路径从 repo 内 `.pids` 统一迁移到 `<config_dir>/pids`；Lark main/test/supervisor 状态输出补充 config 与 pid_dir 打印。
- 2026-02-10: `tests/scripts/lark-identity-lock.sh`、`tests/scripts/lark-supervisor-smoke.sh`、`go test ./cmd/alex -count=1`、`go test ./internal/devops/...`、`go test ./internal/shared/config -count=1`、`./dev.sh lint` 通过；`./dev.sh test` 存在仓库内既有失败（与本次改动无直接关联）。
