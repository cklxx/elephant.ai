# 2026-02-07 日志可视化页面 + 一键启动脚本

## 目标
- 新增一个可视化日志分析页面，支持查看最近 `log_id`、一键展开对应日志详情并做基础分析。
- 提供一键命令：自动启动本地环境并打开日志分析页面。

## 范围
- 后端：增加 dev 日志索引接口（最近 `log_id` 列表 + 简要统计）。
- 前端：新增 `/dev/log-analyzer` 页面并接入索引/详情 API。
- 脚本：扩展 `dev.sh` 增加一键入口（启动 + 打开页面）。

## 方案
1. 在 `internal/shared/logging` 增加日志索引能力：
   - 聚合 `alex-service.log` / `alex-llm.log` / `alex-latency.log` / `llm.jsonl`。
   - 输出按最近时间排序的 `log_id` 列表与计数统计。
2. 在 dev API 增加新端点：
   - `GET /api/dev/logs/index?limit=N`
3. Web 新增日志分析页：
   - 最近日志列表（可筛选）
   - 点击 `log_id` 后加载现有 `GET /api/dev/logs?log_id=...` 详情
   - 展示 Service / LLM / Latency / Requests 四类片段与汇总
4. 增加一键脚本命令：
   - `./dev.sh logs-ui`（启动服务并自动打开 `/dev/log-analyzer`）

## 验证
- Go 单测：
  - 日志索引聚合逻辑测试
  - 新 dev API handler 测试
- Web 单测/类型检查（如需要）
- 全量回归：
  - `go test ./...`
  - `./scripts/run-golangci-lint.sh run ./...`

## 验证结果
- `go test ./internal/shared/logging ./internal/delivery/server/http` 通过。
- `npm --prefix web ci` 后 `npm --prefix web run -s lint` 通过。
- `go test ./...` 通过。
- `./scripts/run-golangci-lint.sh run ./...` 失败，存在仓库既有问题（`evaluation/task_mgmt/*`, `evaluation/rl/storage_test.go`, `internal/delivery/eval/http/api_handler_rl.go` 的 `errcheck/unused`），与本次改动无关。

## 进度
- [x] 新建计划
- [x] 实现日志索引与测试
- [x] 实现 dev API 与测试
- [x] 实现前端可视化页面
- [x] 实现 `dev.sh` 一键命令
- [x] 运行 lint/tests 并修复
- [x] 更新计划状态并提交
