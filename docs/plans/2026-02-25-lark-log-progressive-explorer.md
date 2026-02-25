# 2026-02-25 Lark Log Progressive Explorer

## Goal
- 查明最近一次 Lark agent 长时间无返回的根因。
- 提供一个可复用的“渐进式日志探索”工具，支持从最新消息逐步钻取到会话、执行、失败与交付链路。

## Constraints
- 仅使用仓库内已有日志文件（默认 `logs/alex-service.log`，辅以 `logs/lark-main.log`、`logs/backend.log`）。
- 输出聚焦可操作诊断，不做泛泛摘要。
- 工具可在本地直接运行，无额外依赖。

## Plan
1. `completed`：梳理最近一次 Lark 消息的完整链路并确定根因证据。
2. `completed`：实现渐进式探索脚本（latest → routing → execution → delivery → diagnosis）。
3. `completed`：用当前日志验证脚本输出，确保能命中本次问题。
4. `completed`：补充使用说明并给出本次事件结论。

## Validation
- `python3 scripts/analysis/lark_log_explorer.py --show-candidates --show-evidence`
- `python3 scripts/analysis/lark_log_explorer.py --message-id om_x100b56dbf72cb8a8c38dd8b12cc2112 --show-evidence`
