# 2026-03-10 Lark SDK Audit

- [x] 创建 worktree
- [x] 审计 `internal/infra/lark/` 实现、测试与调用点
- [x] 删除 dead code、简化卡片/消息构建逻辑
- [x] 回归验证
- [ ] review / commit / merge

## Notes

- 删除未被任何生产或测试代码引用的 `Client.Raw()` 与 `PermissionService` 包装层。
- 抽出 task patch 共享 helper，消除 task 创建/更新/完成路径中的重复 request builder 逻辑。
- 抽出 mail group 统一解析 helper，避免 `Get` / `Create` / `List` 三套重复字段映射。
- 将 `summary` 的摘要模板拆成 header / participants / highlights / activity 片段，收敛“卡片摘要”文本构建逻辑。
- 验证已通过：`go test ./internal/infra/lark/...`、`./scripts/run-golangci-lint.sh run ./internal/infra/lark/...`
