# 2026-03-03 · 服务就绪结论先于验证

## Context
- 在连续修复与模拟后，先给出“可继续联调”的状态判断。
- 随后被用户指出服务并未全部启动。

## Symptom
- 结论与实际运行态不一致，导致沟通成本上升并打断排障节奏。

## Root Cause
- 汇报前缺少固定的服务就绪核验清单（进程、端口、健康检查）。
- 把“命令执行成功”误当成“服务可用”。

## Remediation
- 在汇报“已启动/已模拟完成”前，必须完成并记录：
  - 进程存在：`ps` 或 supervisor status
  - 端口监听：`lsof -iTCP -sTCP:LISTEN`
  - 健康探针：`curl /health`（或对应服务健康端点）
- 若任一项未通过，明确标注“未就绪”并附失败原因。

## Follow-up
- 将该核验步骤作为默认收尾动作，适用于 backend/web/lark 三类服务。

## Metadata
- id: err-2026-03-03-service-readiness-claim-without-verification
- tags: [service-readiness, verification, devops, communication]
- links:
  - docs/error-experience/summary/entries/2026-03-03-service-readiness-claim-without-verification.md
  - docs/plans/2026-03-03-hooks-memory-root-cause-and-guardrails.md
