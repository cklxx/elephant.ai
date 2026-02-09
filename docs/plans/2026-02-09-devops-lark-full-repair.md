# 2026-02-09 — DevOps Lark/AuthDB 全量修复

## 目标
- 一次性修复 Lark/AuthDB 与 Go devops 进程链路中的稳定性问题，恢复本地启动与测试稳定性。

## 范围
- `internal/shared/config/runtime_watcher.go` 并发竞态修复
- `internal/devops/process/manager.go` 进程追踪并发与 PID 身份校验
- `internal/devops/supervisor/*` 重启阈值语义统一与调度阻塞修复
- `cmd/alex/dev_lark.go` 启动/异常路径 PID 一致性

## 验收
- `./dev.sh lint` 通过
- `./dev.sh test` 通过
- `./tests/scripts/lark-supervisor-smoke.sh` 通过
- `./tests/scripts/lark-autofix-smoke.sh` 通过

## 里程碑
- [x] M0: 基线与规则检查（engineering practices + memory）
- [x] M1: Runtime watcher race 修复 + 测试
- [ ] M2: Process manager 并发/PID 语义修复 + 测试
- [ ] M3: Restart policy/supervisor 修复 + 测试
- [ ] M4: Lark CLI 启动 PID 一致性修复 + 测试
- [ ] M5: 全量验证、文档沉淀、增量提交、合并回 main

## 风险与约束
- 保持内部行为修复，不引入外部接口破坏。
- PID 身份校验使用强校验，遇到不匹配优先清理陈旧 PID 并拒绝误杀。
