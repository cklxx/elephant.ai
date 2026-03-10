# 2026-03-10 JSON / Errors Audit

- [x] 创建 worktree
- [x] 审计 `internal/shared/json/` 和 `internal/shared/errors/` 实现、测试与调用点
- [x] 删除 dead code、未使用错误分类并简化 JSON / error 逻辑
- [x] 回归验证
- [ ] review / commit / merge

## Notes

- `internal/shared/errors` 中的 `ErrorType` / `GetErrorType` 只有包内测试引用，可删除。
- `RetryStats` / `RetryWithStats` / `ShouldRetry` 也只被测试覆盖使用，属于 dead code。
- `internal/shared/json/jsonx.go` 的 panic fallback 保留，但实现可收敛成单一路径，避免导出函数变量和重复逻辑。
- `FormatForLLM` 可用单一消息提取路径代替三段重复 `errors.As(...)`。
- 验证已通过：`go test ./internal/shared/json ./internal/shared/errors`、`./scripts/run-golangci-lint.sh run ./internal/shared/json/... ./internal/shared/errors/...`、`python3 skills/code-review/run.py review`
