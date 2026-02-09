# Plan: 修复 lark loop 自锁导致 test SHA 长期落后

> Created: 2026-02-09
> Status: completed
> Trigger: 用户反馈 `./alex dev lark status` 显示 test 长期停留旧 SHA，cycle 持续 skipped/merge。

## Goal & Success Criteria
- **Goal**: 修复 `scripts/lark/loop.sh` 的 lock 生命周期，使 watch 模式每轮 cycle 后正确释放锁。
- **Done when**:
  - watch 模式不会重复输出 `Loop already running (lock ...)` 自锁日志。
  - 当 main 有新 SHA 时，loop 能继续执行下一轮 cycle（不被同进程遗留锁阻塞）。
  - `./alex dev lark status` 中 test SHA 可在后续 cycle 后追到 main（或正常进入新一轮验证流程）。
- **Non-goals**:
  - 不重构 supervisor 状态机。
  - 不调整 fast/slow gate 策略。

## Current State
- `run_cycle` 在获取锁后设置 `trap release_lock EXIT`，该 trap 在 watch 长生命周期进程中不会于函数返回触发。
- 一旦某轮 cycle 提前返回（如 `main moved during cycle`），锁目录保留，后续轮次全部命中“锁已存在”，形成自锁。
- 结果是 `last_processed_sha` 长期空值，test 分支无法继续推进。

## Task Breakdown

| # | Task | Files | Size | Depends On |
|---|------|-------|------|------------|
| 1 | 修复 run_cycle 锁释放语义（每轮必释放） | scripts/lark/loop.sh | M | — |
| 2 | 增加回归验证步骤（watch/re-run 行为） | scripts/lark/loop.sh, runtime logs | S | T1 |
| 3 | 运行 lint/test 保障无回归 | repo-wide | M | T1 |

## Technical Design
- **Approach**:
  - 将 `run_cycle` 拆分为外层锁管理函数与内层执行函数：
    - 外层函数负责 `acquire_lock` / `trap` 安装与清理 / `release_lock`。
    - 内层函数保留现有流程和返回码语义。
  - 外层在内层返回后无条件清理 trap 与 lock，确保 watch 下一轮可继续。
  - 保留异常退出场景的 `EXIT` 兜底清理（避免真实崩溃后锁泄露）。
- **Alternatives rejected**:
  - 仅在每个 return 前手动 `release_lock`：遗漏路径风险高，维护性差。
  - 用 RETURN trap：会在子函数返回时提前触发，不符合预期。
- **Key decisions**:
  - 采用“内外层分离”最小侵入修复，避免大改 cycle 逻辑。

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| 修改返回码路径导致 watch 判断偏差 | M | M | 保持内层返回码不变，外层仅做清理 |
| 清理时序不当造成并发竞态 | L | M | 锁仅由 run_cycle 外层管理，单点清理 |
| 本地已有旧锁影响验证结论 | M | M | 验证前显式停止 loop-agent 并清理旧锁 |

## Verification
- 功能验证：
  - `tests/scripts/lark-loop-lock-release.sh`（新增回归测试，PASS）
  - `tests/scripts/lark-supervisor-smoke.sh`（现有 smoke，PASS）
- 质量验证：
  - `./dev.sh lint`（PASS）
  - `./dev.sh test`（FAIL，仓库既有 race：`internal/delivery/channels/lark/gateway_test.go:1061`，与本次改动无关）
- 回滚：
  - 回退 `scripts/lark/loop.sh` 本次改动。

## Progress Log
- 2026-02-09 20:33：定位根因为 `run_cycle` 使用 `trap ... EXIT` 仅在进程退出时释放锁，导致 watch 模式函数返回后锁残留。
- 2026-02-09 20:36：将 `run_cycle` 拆分为外层锁管理 + 内层执行，外层确保每轮都执行 `trap - EXIT` 和 `release_lock`。
- 2026-02-09 20:38：新增 `tests/scripts/lark-loop-lock-release.sh` 回归脚本，覆盖“同进程连续 run_cycle 调用”场景，验证不再自锁。
