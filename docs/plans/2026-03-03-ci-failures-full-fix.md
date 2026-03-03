# 2026-03-03 — CI 报错全部修复

## 背景

用户要求：修复当前分支上的全部 CI 报错。

当前上下文：
- 分支：`main`
- 工作区已有未提交改动（14 files），需避免覆盖非本次修复相关内容。
- 已完成 pre-work checklist 与工程规范/记忆加载。

## 目标

1. 在本地稳定复现 CI 失败。
2. 修复所有导致 CI 失败的问题（按失败顺序逐项清理）。
3. 通过仓库标准质量门禁。

## 执行策略

1. 跑标准门禁：`./scripts/pre-push.sh`，记录首个失败点。
2. 做最小必要修复，仅触及失败相关文件。
3. 每个修复点完成后，先跑最小验证（对应包 test/lint），再回归全量门禁。
4. 全量通过后，执行代码审查脚本并处理 P0/P1。

## 验证命令

- `./scripts/pre-push.sh`
- 必要时补充对应子集命令（如 `go test ./<pkg> -count=1`）。
- `python3 skills/code-review/run.py '{"action":"review"}'`

## 进度

- [x] Pre-work checklist（`git diff --stat`, `git log --oneline -10`）
- [x] 工程规范与记忆条目加载
- [x] 复现 CI 失败
- [x] 修复失败项
- [x] 全量门禁通过
- [x] 代码审查与收尾

## 结果与根因

1. 远端主 CI（`🧪 CI Pipeline`）失败点：
   - `go test -race ./...` 中 `internal/infra/external/workspace` 的 `TestMergeConflictPopulatesConflictDiff` 不稳定失败。
2. 远端静态导出构建失败点（Pages 流程）：
   - `/robots.txt` 与 `/sitemap.xml` 在 `output: export` 模式下缺少静态导出声明。

## 修复记录

1. `internal/infra/external/workspace/manager_test.go`
   - 冲突测试改用专用文件 `merge-conflict-fixture.txt`，避免 `README.md` 受环境 merge 属性影响。
   - 在测试仓库初始化时写入本地 `git` 用户身份，消除 runner 无全局 identity 的不确定性。
2. `web/app/robots.ts`、`web/app/sitemap.ts`
   - 增加 `dynamic = "force-static"` 与 `revalidate = 3600`，满足 `output: export` 对 metadata route 的静态要求。

## 验证结果

- `go test -race ./internal/infra/external/workspace -count=1` ✅
- 模拟 CI git 环境下：
  - `HOME=<tmp> GIT_CONFIG_GLOBAL=<tmp> go test -race ./internal/infra/external/workspace -run TestMergeConflictPopulatesConflictDiff -count=10` ✅
- `STATIC_EXPORT=1 NEXT_PUBLIC_BASE_PATH=/elephant.ai NEXT_PUBLIC_ASSET_PREFIX=/elephant.ai npm --prefix web run build`
  - `robots/sitemap` 错误已消失；仅剩 API route 与静态导出的已知不兼容（Pages 流程会移除 `web/app/api`）。
- `./scripts/pre-push.sh` ✅
- `python3 skills/code-review/run.py '{"action":"review"}'` ✅
