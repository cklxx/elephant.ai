# 2026-02-25 Kernel Context 外溢修复与复盘机制建设

## 目标
- 修复 `KernelAlignmentContext` 被注入普通 Lark 会话的问题，确保仅在 unattended/kernel 执行路径注入。
- 为该事故补齐结构化复盘记录，并沉淀防回归方案。
- 系统化建立复盘机制目录、模板、流程，支持后续同类问题快速归档与复用。

## 范围
- 代码：`internal/app/agent/preparation/`（注入时机控制 + 单测）
- 记录：`docs/error-experience/`、`docs/good-experience/`、`docs/postmortems/`
- 机制：新增 `docs/postmortems/` 目录体系与模板

## 执行步骤
1. [completed] 修复注入边界：仅 unattended 上下文注入 kernel alignment context。
2. [completed] 增加测试：覆盖非 unattended 不注入、unattended 注入两条路径。
3. [completed] 编写本次事故复盘记录（根因、影响、修复、预防）。
4. [completed] 建立通用复盘机制目录与模板（流程、检查表、模板）。
5. [completed] 运行 lint/test 与 code-review 技能检查，完成提交。

## 验证命令（计划）
- `go test ./internal/app/agent/preparation -run KernelAlignment -count=1`
- `go test ./...`
- `make lint`
- `python3 skills/code-review/run.py '{"action":"review"}'`

## 风险与回滚
- 风险：误伤 kernel unattended 路径，导致 kernel 失去对齐上下文。
- 缓解：通过“正反两条测试”锁定边界；仅改注入时机，不改内容格式。
- 回滚：单文件回滚 `internal/app/agent/preparation/service.go` 与新增测试。
