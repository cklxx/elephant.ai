# 2026-03-05 Migrate Lark Builtins To Skills

## Goal
将内置飞书工具实现从 `internal/infra/tools/builtin/larktools/*` 迁移到 skill/CLI 运行时，彻底移除 `channel` 注册入口，统一改为 `shell_exec` + `skills/*/run.py` + `scripts/cli/feishu/feishu_cli.py`。

## Plan
- [completed] 扩展 feishu-cli：补齐 message/task/doc/calendar 缺失 action，保持旧 action 兼容别名。
- [completed] 删除旧 larktools 实现与测试（calendar/task/doc/wiki/drive/sheets/okr/mail/contact/bitable/vc/message/upload）。
- [completed] 从 registry 移除 `channel` 工具注册与依赖，更新受影响测试与提示词路由。
- [completed] 执行测试：Go 单测 + Python skill/cli 测试 + 真实请求抽样验证（无 channel 路径）。
- [pending] 提交变更并合并回 `main`（ff-only），清理 worktree。

## Risks
- 旧测试强依赖 `channel` 路由；移除注册后需重写为“shell_exec + skill/cli 调用”语义测试。
- 部分动作在不同租户权限下会失败，需要在真实验证中记录“权限不足但调用路径正确”。

## Verification
- `go test ./internal/app/toolregistry ./internal/app/agent/preparation ./internal/app/context ./internal/delivery/channels/... ./internal/domain/agent/presets ./internal/app/scheduler`
- `python3 -m pytest -q skills/feishu-cli/tests`
- 真实请求：`python3 skills/feishu-cli/run.py ...` 按模块抽样执行。
