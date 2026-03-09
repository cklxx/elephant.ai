# Autonomous Audit — docx create-doc path and lark boundary drift

- Timestamp: 2026-03-08T00:39:21Z
- Scope: `internal/infra/tools/builtin/larktools`, `internal/infra/lark`, `STATE.md`
- Goal: verify current docx create-doc reality vs STATE/history, and check whether larktools↔infra/lark boundaries show stale references, wrong verification targets, or scoped lint/test drift.

## Bottom line

1. **docx create-doc current real package path is unchanged and explicit**: tool entry stays in `internal/infra/tools/builtin/larktools/docx_manage.go`, SDK wrapper stays in `internal/infra/lark/docx.go`.
2. **Current targeted tests are passing now**; the formerly risky create-doc-with-initial-content path is green in both package layers.
3. **STATE.md is currently stale for this topic**: it does not mention the later docx convert-route revalidation nor today’s recheck, so historical STATE under-describes current reality.
4. **No fresh boundary drift found** between `larktools` and `internal/infra/lark` in the audited area. The boundary is still: channel/tool dispatch in `larktools`, typed Lark SDK calls in `internal/infra/lark`.
5. **Scoped lint/test baseline is stable** for the audited packages. I did not find evidence that current green results are accidentally targeting the wrong package or an obsolete route.

## Evidence

### Real code path

- `internal/infra/tools/builtin/larktools/docx_manage.go:23` wraps SDK client with `larkapi.Wrap(sdkClient)`.
- `internal/infra/tools/builtin/larktools/docx_manage.go:45-67` implements `createDoc(...)` and calls:
  - `client.Docx().CreateDocument(...)`
  - `client.Docx().WriteMarkdown(ctx, doc.DocumentID, doc.DocumentID, content)` when initial content is present.
- `internal/infra/lark/docx.go:233-253` implements `ConvertMarkdownToBlocks(...)`.
- `internal/infra/lark/docx.go:311-312` (via grep output) keeps `WriteMarkdown(...)` in `internal/infra/lark`, consuming `ConvertMarkdownToBlocks(...)`.

### Current test status

#### Scoped create-doc tests (`larktools`)
Command:
```bash
go test -count=1 ./internal/infra/tools/builtin/larktools -run 'TestDocxManage_CreateDoc|TestDocxManage_CreateDoc_WithInitialContent|TestDocxManage_CreateDoc_WithInitialContent_UsesServerConvertMock|TestDocxManage_CreateDoc_WithFolder|TestDocxManage_CreateDoc_APIError' -v
```
Result:
- `TestDocxManage_CreateDoc` ✅
- `TestDocxManage_CreateDoc_WithInitialContent` ✅
- `TestDocxManage_CreateDoc_WithInitialContent_UsesServerConvertMock` ✅
- `TestDocxManage_CreateDoc_WithFolder` ✅
- `TestDocxManage_CreateDoc_APIError` ✅

#### Scoped docx SDK tests (`internal/infra/lark`)
Command:
```bash
go test -count=1 ./internal/infra/lark -run 'TestCreateDocument|TestConvertMarkdownToBlocks|TestUpdateDocumentBlockText|TestBuildDocumentURL' -v
```
Result:
- `TestCreateDocument` ✅
- `TestConvertMarkdownToBlocks` ✅
- `TestUpdateDocumentBlockText` ✅
- `TestBuildDocumentURL` ✅
- Other selected docx package tests in the run also passed.

#### Package baseline recheck
Command:
```bash
go test -count=1 ./internal/infra/tools/builtin/larktools ./internal/infra/lark
```
Result:
- `ok  alex/internal/infra/tools/builtin/larktools  0.622s`
- `ok  alex/internal/infra/lark  (cached)`

### Route/fixture verification

Current local diff in `internal/infra/tools/builtin/larktools/docx_manage_test.go` is **test-only** and tightens verification rather than changing runtime logic:
- asserts convert request contains `"content_type":"markdown"`
- asserts convert request contains `"content":`
- accepts both `/open-apis/docx/v1/documents/blocks/convert` and `/docx/v1/documents/blocks/convert`
- validates trailing-slash variants
- adds `block_id_to_image_urls: []` to mocked convert response

This aligns with the actual wrapper contract in `internal/infra/lark/docx.go` and reduces mock-shape drift.

### Boundary audit

Evidence from grep/readback shows:
- `internal/infra/tools/builtin/larktools/channel.go` still exposes document actions (`create_doc`, `read_doc`, `read_doc_content`, `list_doc_blocks`, `update_doc_block`, `write_doc_markdown`) from the unified channel layer.
- `internal/infra/tools/builtin/larktools/docx_manage.go` remains the tool-layer adapter.
- `internal/infra/lark/docx.go` remains the SDK/transport abstraction.

I did **not** find:
- a migrated/new docx package path superseding these files,
- stale tool code still pointing at removed docx APIs,
- verification commands aimed at the wrong package,
- scoped lint drift in this audited area.

### Lint baseline
Command:
```bash
golangci-lint --version
golangci-lint run ./internal/infra/tools/builtin/larktools/... ./internal/infra/lark/...
```
Result:
- `golangci-lint` present: `v1.64.8`
- scoped lint returned clean output in this run.

## Consistency vs STATE/history

### With historical reports
Today’s results are consistent with the later historical reports under `docs/reports/` that already claimed the convert-route issue was closed, especially:
- `docs/reports/kernel-cycle-2026-03-06T04-08Z-lark-docx-convert-revalidation.md`
- `docs/reports/larktools-lint-backlog-audit-20260306T040917Z.md`
- `docs/reports/build-executor-adjacent-autonomous-audit-20260308T000925Z.md`

### With `STATE.md`
Not consistent enough. `STATE.md` currently only tracks push/network and task-store findings; it does **not** capture that:
- the docx create-doc convert-route path was revalidated after the earlier failure concern,
- targeted tests are now green,
- the remaining workspace delta in this area is test-only.

## Recommendation on `STATE.md`

**Yes — update `STATE.md`.** Suggested addition:
- docx create-doc path remains `larktools/docx_manage.go -> internal/infra/lark/docx.go`
- scoped tests for create-doc + convert/write flow are green
- scoped lint for `larktools` + `internal/infra/lark` is green
- working tree is still dirty because `internal/infra/tools/builtin/larktools/docx_manage_test.go` has uncommitted test-harness tightening

## Evidence commands executed

```bash
git status --short
rg -n 'docx|create.?doc|ConvertMarkdownToBlocks|WriteMarkdown|package lark|package larktools' internal/infra/tools/builtin/larktools internal/infra/lark STATE.md docs/reports

go test -count=1 ./internal/infra/tools/builtin/larktools -run 'TestDocxManage_CreateDoc|TestDocxManage_CreateDoc_WithInitialContent|TestDocxManage_CreateDoc_WithInitialContent_UsesServerConvertMock|TestDocxManage_CreateDoc_WithFolder|TestDocxManage_CreateDoc_APIError' -v

go test -count=1 ./internal/infra/lark -run 'TestCreateDocument|TestConvertMarkdownToBlocks|TestUpdateDocumentBlockText|TestBuildDocumentURL' -v

go test -count=1 ./internal/infra/tools/builtin/larktools ./internal/infra/lark

golangci-lint --version
golangci-lint run ./internal/infra/tools/builtin/larktools/... ./internal/infra/lark/...

git diff -- internal/infra/tools/builtin/larktools/docx_manage_test.go
```

## Residual risk

- The workspace is still dirty, so future audits can confuse “already-fixed locally” with “committed baseline”.
- `STATE.md` omission is now the main documentation drift; code/test drift in the audited area looks low.

