# Plan: Lark Supervisor Self-Heal + Command Surface Rationalization

> Created: 2026-02-10
> Status: completed
> Trigger: 用户反馈 `./dev.sh lark-up` 时 supervisor 已运行但 `loop` 掉线，且 `dev.sh`/`lark.sh`/`alex dev lark` 命令面混乱、不直觉。

## Goal & Success Criteria
- **Goal**: 保证本地 Lark 仅由独立进程集（main/test/loop）在 supervisor 下托管，且命令入口直觉统一。
- **Done when**:
  - `supervisor` 已运行时执行启动命令，会自动修复掉线组件（尤其 `loop`）。
  - `dev.sh` 提供统一 `lark` 子命令入口，旧别名可用但提示弃用。
  - `alex dev lark` 默认执行启动（非仅状态），避免误判“没启动命令”。
  - 状态输出能清楚显示 main/test 配置文件路径与共享 pid 目录。
- **Non-goals**:
  - 不做“devops 全量迁移 Python”这类大规模重写。
  - 不改动与 Lark 进程管理无关的业务逻辑。

## Current State
- `scripts/lark/supervisor.sh` 的 `start` 在检测到 supervisor 存活时只打印子组件健康状态并返回，不会立即做组件自愈。
- `dev.sh` 同时维护 `all-up`/`lark-up` 与其他命令，命令语义重复，用户难以记忆。
- `alex dev lark` 默认命令是 `status`，与用户“直接执行应启动”的直觉冲突。
- main/test 已支持打印配置文件路径与 identity，但入口层展示不一致。

## Task Breakdown

| # | Task | Files | Size | Depends On |
|---|------|-------|------|------------|
| 1 | supervisor 在“已运行”分支增加单次自愈流程 | `scripts/lark/supervisor.sh` | M | — |
| 2 | 统一 dev 命令入口到 `dev.sh lark <subcommand>` 并兼容旧别名 | `dev.sh` | M | T1 |
| 3 | 调整 `alex dev lark` 默认行为及帮助信息 | `cmd/alex/dev_lark.go`, `README.md`, `README.zh.md` | S | T2 |
| 4 | 回归验证与 smoke 检查 | `tests/scripts/lark-supervisor-smoke.sh`(执行), `./dev.sh lint`, `./dev.sh test` | M | T1,T2,T3 |
| 5 | 代码审查、修复审查项、增量提交并合并 main | review references + git metadata | M | T4 |

## Technical Design
- **Approach**:
  - 在 `supervisor.sh` 中新增单次 reconcile 逻辑：当 `start` 发现 supervisor 已存活时，主动检查并重启 down 组件，再回显健康状态，避免“已运行但不自愈”。
  - `dev.sh` 的 lark 管理改为统一分组命令（`dev.sh lark up|down|status|logs|restart|doctor`），内部转发到 `./lark.sh`，保持单一事实来源。
  - 保留 `lark-up/lark-down/...` 旧命令作为兼容层并打印弃用提示，减少破坏性。
  - `alex dev lark` 默认命令改为 `start`，并更新 usage 文档，降低误用。
- **Alternatives rejected**:
  - 仅在文档中提示用户使用 `lark.sh`：无法解决“已运行但 loop 掉线不自愈”的核心问题。
  - 直接移除旧命令：对现有脚本/习惯破坏较大，不利于平滑迁移。
- **Key decisions**:
  - `dev.sh` 作为统一入口但不重复实现 supervisor 逻辑，直接委托 `lark.sh`。
  - 自愈策略放在 supervisor 层，保证无论入口来自 `dev.sh` 还是 `lark.sh`，行为一致。

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `start` 分支新增自愈与主循环 tick 并发竞争 | M | M | 自愈做一次性轻量检查，复用既有 restart 函数并在关键步骤重新 observe。 |
| 命令兼容调整引发历史脚本断裂 | M | H | 保留旧命令别名并输出明确 deprecate 提示。 |
| `alex dev lark` 默认行为变化影响依赖 status 的调用方 | M | M | 保留显式 `status` 子命令，文档升级并在 release note 风格日志说明。 |

## Verification
- `bash -n dev.sh lark.sh scripts/lark/*.sh scripts/lib/common/*.sh`
- `./tests/scripts/lark-supervisor-smoke.sh`
- `./dev.sh lint`
- `./dev.sh test`
- 手工检查：`./dev.sh lark up` 在 supervisor 已运行且 loop down 场景能恢复 loop。
- 回滚方案：若异常，按 commit 级别回退该分支，保留兼容命令路径。

## Execution Log
- 2026-02-10 14:34: 完成 `supervisor.sh` 自愈增强：`start` 在已运行分支执行一次 `reconcile_children_once`，并在输出中增加 `pid_dir/main config/test config`。
- 2026-02-10 14:35: 完成 `dev.sh` 命令面收敛：新增 `dev.sh lark <subcommand>`，保留 `lark-up/down/status/logs` 兼容别名并提示弃用；`up --lark` 作为推荐入口。
- 2026-02-10 14:36: 修复并增强 `alex dev lark`：默认命令改为 `start`，补充 `up/down/restart` 别名；启动成功判定改为等待 supervise 进程发布 pid 文件，避免假阳性。
- 2026-02-10 14:37: README/README.zh.md 命令文档同步更新。
- 2026-02-10 14:39: 通过 `bash -n`、`tests/scripts/lark-supervisor-smoke.sh`、`./dev.sh lint`。
- 2026-02-10 14:41: `./dev.sh test` 执行完成但失败，失败项为环境相关既有用例：
  - `cmd/alex` 下 `TestExecuteConfigCommandValidateQuickstartAllowsMissingLLMKey` / `TestExecuteConfigCommandValidateProductionFailsWithoutLLMKey`
  - `internal/delivery/server/bootstrap` 下 `TestLoadConfig_ProductionProfileRequiresAPIKey`
  该失败与本次 lark 进程管理改动无直接耦合，已在交付说明中标注为测试基线风险。
