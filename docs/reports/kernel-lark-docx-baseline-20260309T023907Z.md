# Kernel focused audit: lark/docx validation baseline

- Timestamp (UTC): 20260309T023907Z
- Repo: `/Users/bytedance/code/elephant.ai`
- Branch: `main`
- HEAD: `1dfdbcc580ebe7f7edcd7c0b86135964d81e1b20` (`1dfdbcc5`)

## Scope
Provide an executable baseline for current `lark/docx` test repair work, focused on `internal/infra/tools/builtin/larktools` docx create-doc and markdown-to-blocks convert coverage.

## 1) Git state and uncommitted work
Observed via `git status --short`:
- Modified: `STATE.md`
- Modified: `internal/infra/tools/builtin/larktools/docx_manage_test.go`
- Multiple untracked files already present under `docs/plans/` and `docs/reports/`

Relevant uncommitted diff in `internal/infra/tools/builtin/larktools/docx_manage_test.go`:
- Adds stricter convert request assertions (`content_type=markdown`, non-empty content)
- Adds supported convert route helper for both `/open-apis/.../convert` and `/docx/.../convert`, including trailing slash variants
- Expands mock convert response to include `block_id_to_image_urls: []`
- Tightens descendant payload assertions in end-to-end channel tests

## 2) Implementation and test entry points
### Main implementation
- `internal/infra/tools/builtin/larktools/docx_manage.go:45-67`
  - `createDoc()` creates a document, then if `content/description` is non-empty calls `client.Docx().WriteMarkdown(ctx, doc.DocumentID, doc.DocumentID, content)`
- `internal/infra/tools/builtin/larktools/docx_manage.go:182-209`
  - `writeMarkdown()` lists document blocks, finds page block, then calls `client.Docx().WriteMarkdown(...)`
- `internal/infra/lark/docx.go:231-268`
  - `ConvertMarkdownToBlocks()` builds the convert request and calls SDK route `Docx.V1.Document.Convert`
- `internal/infra/lark/docx.go:311-345`
  - `WriteMarkdown()` depends on `ConvertMarkdownToBlocks()` first, then inserts descendants

### Test entry points in `internal/infra/tools/builtin/larktools/docx_manage_test.go`
- Convert-route mock helpers:
  - `larkTestServerWithDocxConvertMock()`
  - `larkTestServerWithObservedDocxConvertMock()`
  - `isDocxBlocksConvertRoute()` / `isSupportedDocxConvertPath()`
  - `writeDocxConvertSuccess()`
- Direct docx manage tests:
  - `TestDocxManage_CreateDoc_WithInitialContent`
  - `TestDocxManage_CreateDoc_WithInitialContent_UsesServerConvertMock`
- Channel/E2E tests:
  - `TestChannel_CreateDoc_WithContent_E2E`
  - `TestChannel_CreateDoc_WithContent_UsesDefaultConvertMockRoute`
  - `TestChannel_CreateDoc_WithContent_ConvertAPIError`
- Upstream lark client tests:
  - `internal/infra/lark/docx_test.go: TestConvertMarkdownToBlocks`, `TestConvertMarkdownToBlocksAPIError`, `TestListDocumentBlocks`

## 3) Commands executed and factual results
### Git / workspace
```bash
git rev-parse HEAD
git status --short
git branch --show-current
git diff -- internal/infra/tools/builtin/larktools/docx_manage_test.go
```
Result:
- HEAD is `1dfdbcc580ebe7f7edcd7c0b86135964d81e1b20`
- Current branch is `main`
- There is uncommitted work, including the target test file

### Discovery
```bash
rg -n "create-doc|docx|markdown-to-blocks|convert route|convert" internal/infra/tools/builtin/larktools -S
rg -n "WriteMarkdown\(|blocks/convert|ConvertDocument|ListDocumentBlocks\(" internal/infra/lark internal/infra/tools/builtin/larktools -S
```
Result:
- Confirmed `channel.go` dispatches docx actions to `larkDocxManage`
- Confirmed `docx_manage.go` create/write path depends on `client.Docx().WriteMarkdown`
- Confirmed `internal/infra/lark/docx.go` convert path is real and current

### Minimal validation tests
```bash
go test ./internal/infra/tools/builtin/larktools -run 'Test(DocxManage_CreateDoc_WithInitialContent|DocxManage_CreateDoc_WithInitialContent_UsesServerConvertMock|LarkTestServerWithDocxConvertMock_HandlesConvertRoutes|Channel_CreateDoc_WithContent_UsesDefaultConvertMockRoute|Channel_CreateDoc_WithContent_ConvertAPIError)$' -v
```
Result:
- All targeted larktools docx/convert tests passed

```bash
go test ./internal/infra/tools/builtin/larktools -run 'Test(DocxManage_|Channel_CreateDoc_|Docx_FullLifecycle)' -v
```
Result:
- Package-level focused docx create/read/write lifecycle coverage passed

```bash
go test ./internal/infra/lark -run 'Test.*(Convert|WriteMarkdown|ListDocumentBlocks)' -v
```
Result:
- `TestConvertMarkdownToBlocks` passed
- `TestConvertMarkdownToBlocksAPIError` passed
- `TestListDocumentBlocks` passed

## 4) Bottom-line audit conclusion
### Is the current failure still caused by missing convert route?
No — not in the current workspace baseline.

Why:
- The targeted tests that previously would fail on missing `/docx/v1/documents/blocks/convert` coverage are now green.
- The uncommitted changes in `internal/infra/tools/builtin/larktools/docx_manage_test.go` explicitly add/strengthen convert route coverage, including:
  - both `/open-apis/...` and `/docx/...`
  - optional trailing slash handling
  - stricter convert request validation
  - response shape compatible with current SDK parser (`block_id_to_image_urls` included)
- Upstream `internal/infra/lark` convert tests are also green, so the client-side convert call path itself is not currently the blocker.

### Relevant file paths
- `internal/infra/tools/builtin/larktools/channel.go`
- `internal/infra/tools/builtin/larktools/docx_manage.go`
- `internal/infra/tools/builtin/larktools/docx_manage_test.go`
- `internal/infra/lark/docx.go`
- `internal/infra/lark/docx_test.go`

### Recommended validation commands
Fastest baseline:
```bash
go test ./internal/infra/tools/builtin/larktools -run 'Test(DocxManage_CreateDoc_WithInitialContent|DocxManage_CreateDoc_WithInitialContent_UsesServerConvertMock|LarkTestServerWithDocxConvertMock_HandlesConvertRoutes|Channel_CreateDoc_WithContent_UsesDefaultConvertMockRoute|Channel_CreateDoc_WithContent_ConvertAPIError)$' -v
```
Wider package confidence:
```bash
go test ./internal/infra/tools/builtin/larktools -run 'Test(DocxManage_|Channel_CreateDoc_|Docx_FullLifecycle)' -v
```
Client-layer confirmation:
```bash
go test ./internal/infra/lark -run 'Test.*(Convert|WriteMarkdown|ListDocumentBlocks)' -v
```

## 5) Extra hidden blockers
No additional hidden blocker was confirmed by this audit run.

However, two practical caveats remain:
1. The passing baseline depends on uncommitted local test changes in `internal/infra/tools/builtin/larktools/docx_manage_test.go`; if those edits are reverted or absent in another checkout, the old convert-route failure can reappear.
2. Workspace is dirty, so anyone validating from a clean CI or another machine must first confirm whether this test-file diff is intended to be committed.

## Recommendation
Treat the current baseline as: **convert-route issue locally fixed in tests but not yet committed**. Next safe move is to commit or otherwise preserve `internal/infra/tools/builtin/larktools/docx_manage_test.go`, then rerun the three validation commands above in CI/clean checkout to confirm no environment-specific masking.

