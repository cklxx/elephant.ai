# Build-executor 周边自治审计报告 — 20260308T000925Z

## 范围
- 仓库：`/Users/bytedance/code/elephant.ai`
- 聚焦：`internal/infra/tools/builtin/larktools`、`internal/infra/lark` 及其相关测试
- 目标：避免重复最近已通过的基线验证，只做 build-executor 修复目标周边的最小必要只读核实

## 与 STATE 的差异
当前 `STATE.md`（更新于 2026-03-06 13:47）主要记录：
- team runtime smoke test 通过
- push 受 SSH/network reset 阻塞
- 不应删除/重建 `~/.alex/lark/tasks.json`

本次审计新增且与 STATE 不同的结论：
1. **docx/lark 当前代码路径已稳定落位**，并非待判定状态：
   - `alex/internal/infra/tools/builtin/larktools`
   - `alex/internal/infra/lark`
   - `alex/internal/infra/lark/calendar/meetingprep`
   - `alex/internal/infra/lark/calendar/suggestions`
   - `alex/internal/infra/lark/oauth`
   - `alex/internal/infra/lark/summary`
2. **build-executor 周边的 docx convert-route 风险目前是关闭状态**：
   - `internal/infra/tools/builtin/larktools/docx_manage_test.go` 已明确匹配
     `/open-apis/docx/v1/documents/blocks/convert` 与 `/docx/v1/documents/blocks/convert`
   - mock 返回体已包含 `block_id_to_image_urls: []`
   - scoped `go test` 与 `golangci-lint` 均通过
3. **当前真实工作区仍然 dirty**，这是 STATE 未显式收敛的现实差异：
   - `M STATE.md`
   - `M internal/infra/tools/builtin/larktools/docx_manage_test.go`
   - 多个未跟踪 `docs/reports/*` 报告文件

## 当前实际路径核实
### 包路径
- `internal/infra/tools/builtin/larktools` → Go 包路径：`alex/internal/infra/tools/builtin/larktools`
- `internal/infra/lark` → Go 包路径：`alex/internal/infra/lark`
- 相关子包：
  - `alex/internal/infra/lark/calendar/meetingprep`
  - `alex/internal/infra/lark/calendar/suggestions`
  - `alex/internal/infra/lark/oauth`
  - `alex/internal/infra/lark/summary`

### docx 实现路径
- 统一工具入口：`internal/infra/tools/builtin/larktools/docx_manage.go`
- Lark Docx SDK 封装：`internal/infra/lark/docx.go`
- 文档 URL 构造：`internal/infra/lark/url.go`
- 关键测试：`internal/infra/tools/builtin/larktools/docx_manage_test.go`、`internal/infra/lark/docx_test.go`

### 依赖现状
- `go.mod` 中 Lark SDK 依赖：`github.com/larksuite/oapi-sdk-go/v3 v3.5.3`
- 当前未发现 docx 代码迁移到其他新目录；实现仍集中在上述两处

## 风险扫描结论
### 1) 测试风险
**当前未发现未关闭的红灯。**

证据：
- `go test -count=1 ./internal/infra/tools/builtin/larktools ./internal/infra/lark` → 通过
- `go test -count=1 ./internal/infra/lark/...` → 通过

关键观察：
- `internal/infra/tools/builtin/larktools/docx_manage_test.go` 的本地修改是**测试夹具加强**，不是运行时代码修复：
  - 更严格校验 convert 请求中包含 `"content_type":"markdown"` 和 `"content"`
  - 覆盖带尾斜杠的 convert route
  - 响应体补齐 `block_id_to_image_urls: []`
- `internal/infra/lark/docx.go` 中 `WriteMarkdown(...)` 仍通过：
  - 先 `ConvertMarkdownToBlocks`
  - 再 strip table `merge_info`
  - 再按最多 1000 blocks batching 插入 descendant blocks

结论：docx convert 路径/响应形状与当前实现是对齐的，未见 build-executor 周边仍悬空的单测缺口。

### 2) 路径风险
**当前未发现“包路径漂移”问题。**

- `go list` 返回的包路径与目录结构一致
- `docx_manage.go` 使用 `larkapi "alex/internal/infra/lark"`
- Docx 具体 SDK 调用仍在 `internal/infra/lark/docx.go`

残余注意点：
- `docx_manage.go:createDoc()` 在写入初始内容时直接把 `doc.DocumentID` 作为 page block ID 传给 `WriteMarkdown(...)`
- 同文件 `writeMarkdown()` 分支则会先 `ListDocumentBlocks` 找 page block（`BlockType == 1`），找不到才 fallback 到 `documentID`
- 这两个路径目前测试是绿的，但行为并不完全对称；若后续 Lark API 对新建文档首页 block 约束改变，这里会是首个脆弱点

### 3) lint 风险
**本次 scoped lint 为 green，没有看到新的 lark/docx 邻域 lint backlog。**

证据：
- `golangci-lint run ./internal/infra/tools/builtin/larktools ./internal/infra/lark` → exit 0

说明：
- 这只能说明目标包当前无 lint 红灯；不能替代全仓 lint
- 但按“避免重复最近已通过的基线验证”的要求，这个 scoped gate 已足够形成独立审计结论

### 4) 工作区/交付风险
这是本次最现实的残余风险：
- 工作树非干净，且唯一代码 diff 落在 `internal/infra/tools/builtin/larktools/docx_manage_test.go`
- diffstat：`22 insertions(+), 2 deletions(-)`
- 若 build-executor 修复预期是“runtime code 已收敛”，那当前证据显示**本地仍以测试保障增强为主，不是新的运行时修补**

## 执行证据
### 命令
```bash
pwd && git status --short
find internal/infra/tools/builtin/larktools internal/infra/lark -maxdepth 3 \( -name '*.go' -o -name '*_test.go' \) | sort
go list ./internal/infra/tools/builtin/larktools ./internal/infra/lark/...
rg -n "package lark|package larktools|docx|lark" internal/infra/tools/builtin/larktools internal/infra/lark go.mod
git diff -- internal/infra/tools/builtin/larktools/docx_manage_test.go
go test -count=1 ./internal/infra/tools/builtin/larktools ./internal/infra/lark
go test -count=1 ./internal/infra/lark/...
golangci-lint run ./internal/infra/tools/builtin/larktools ./internal/infra/lark
```

### 关键结果
- `go test -count=1 ./internal/infra/tools/builtin/larktools ./internal/infra/lark`
  - `ok alex/internal/infra/tools/builtin/larktools`
  - `ok alex/internal/infra/lark`
- `go test -count=1 ./internal/infra/lark/...`
  - `ok alex/internal/infra/lark`
  - `ok alex/internal/infra/lark/calendar/meetingprep`
  - `ok alex/internal/infra/lark/calendar/suggestions`
  - `ok alex/internal/infra/lark/oauth`
  - `ok alex/internal/infra/lark/summary`
- `golangci-lint run ./internal/infra/tools/builtin/larktools ./internal/infra/lark`
  - 无输出，exit 0

## 独立审计结论
1. **docx/lark 相关包的当前实际路径明确且稳定**，未发现 build-executor 修复后仍存在的路径漂移。
2. **`internal/infra/tools/builtin/larktools` 与 `internal/infra/lark` 周边当前 scoped tests/lint 均为绿色**，未看到仍未关闭的测试或 lint 红灯。
3. **当前最主要差异不在运行时代码，而在本地测试夹具增强与工作区未清理**；这说明风险更偏向“交付卫生/状态未回写”，而不是“功能仍坏”。
4. **仍建议后续补一条 STATE 更新**：把“docx convert-route 现已在 scoped test + scoped lint 下复核通过，但工作区仍 dirty”写回状态文件；否则 STATE 对这块现状是失真的。

## 建议的下一步（非阻塞）
- 将本次结论回写到 `STATE.md`，尤其是：
  - scoped docx/lark 验证已通过
  - 仍有本地 test-only diff 未收敛
- 若要进一步消除脆弱点，优先统一 `createDoc()` 与 `writeMarkdown()` 对 page block 的解析策略，避免一个路径直接用 `documentID`、另一个路径先探测 page block

