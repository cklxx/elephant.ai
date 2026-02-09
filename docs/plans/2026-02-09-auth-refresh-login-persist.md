# 2026-02-09 登录后刷新仍需重新登录排查与修复

## 背景
- 现象：用户登录成功后，刷新页面仍回到未登录态（需要重新登录）。
- 目标：恢复刷新后会话持续可用，并补齐自动化测试防回归。

## 计划
1. [x] 梳理 auth 链路：登录、refresh、cookie、session 持久层与回退策略。
2. [x] 对比近期改动并复现问题，锁定根因。
3. [x] 以最小改动修复会话持久化/刷新链路。
4. [x] 补充或更新测试（TDD），覆盖刷新保持登录场景。
5. [x] 执行 lint + tests，记录结果与残留风险。
6. [ ] 更新计划和经验记录，提交并合并回 `main`。

## 进度记录
- 2026-02-09 00:55：创建计划，开始定位 auth 刷新链路。
- 2026-02-09 10:10：完成后端 cookie 编解码兼容修复（URL-safe 写入 + 多格式读取）与前端 refresh 失败分级处理（仅鉴权失败清 session）。
- 2026-02-09 10:20：新增回归测试：
  - `internal/delivery/server/http/auth_handler_test.go`：refresh cookie 多编码兼容。
  - `internal/delivery/server/http/middleware_test.go`：URL-safe access cookie 鉴权。
  - `web/lib/auth/client.test.ts`：refresh 瞬时失败不登出、401 仍清会话。
- 2026-02-09 10:30：补查群聊 `apikey not registered` 链路，新增 group-chat 作用域修复（群聊忽略 legacy chat+user fallback，避免 sender 级污染）。
- 2026-02-09 10:35：全量校验结果：
  - ✅ `go test ./internal/delivery/server/http`（定向）
  - ✅ `go test ./internal/delivery/channels/lark`（定向）
  - ✅ `cd web && pnpm exec vitest run --config vitest.config.mts lib/auth/client.test.ts`
  - ⚠️ `./scripts/run-golangci-lint.sh run ./...` 存在既有失败（`internal/devops/*`, `cmd/alex/*` 等 errcheck/unused，非本次改动引入）
  - ⚠️ `go test ./...` 既有失败：`internal/shared/config TestNoUnapprovedGetenv`
  - ⚠️ `cd web && pnpm lint` 既有 warning-as-error（`app/dev/log-analyzer/page.tsx`）
  - ⚠️ `cd web && pnpm test` 既有构建失败（缺少 markdown 相关依赖：`rehype-*`, `remark-gfm`）
