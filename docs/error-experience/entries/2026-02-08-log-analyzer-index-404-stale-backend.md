# 2026-02-08 - /api/dev/logs/index 出现 404（旧后端进程未刷新）

## Error
- 访问 `http://localhost:3000/dev/log-analyzer` 时，前端请求 `GET /api/dev/logs/index?limit=120` 返回 `404 Not Found`。

## Impact
- 日志分析页面无法加载 `log_id` 索引列表，页面仅显示错误提示，无法继续链路分析。

## Root Cause
- 后端代码已包含 `/api/dev/logs/index` 路由，但本地仍在运行旧的 `alex-server` 进程（未重启到新二进制）。
- `./dev.sh logs-ui` 仅做“已运行则复用”，缺少“关键接口可用性探测 + 自动修复”。

## Remediation
- `logs-ui` 增加 readiness probe：
  - 校验 `/api/dev/logs/index` 是否返回 `200/401`（路由存在且鉴权链路可达）。
  - 不可用时自动重启 backend，再次探测；仍失败则立即报错并提示环境需 `development`。
  - 校验 `/dev/log-analyzer` 页面可达，不可用时自动重启 web。
- 前端 `log-analyzer` 页面在 `404/401` 时给出明确行动提示，避免“静默 404”。

## Status
- fixed
