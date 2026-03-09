# Autonomous audit: Lark docx + larktools path verification

Date: 2026-03-08
Repo: `/Users/bytedance/code/elephant.ai`
Scope: verify two current STATE risks with minimal real commands, not full-spectrum research.

## Bottom line
- The `create_doc` -> docx convert chain is still real and covered in the current tree.
- `internal/infra/tools/builtin/larktools` still exists as the tool-facing package; it has **not** been fully migrated away.
- The lower-level Lark API implementation lives in `internal/infra/lark`, and `larktools` now calls into it for docx and several other domains.
- Targeted tests passed and targeted lint is clean for both `internal/infra/lark` and `internal/infra/tools/builtin/larktools`.

## Dirty tree / path reality
`git status --short` showed:
- modified: `STATE.md`
- modified: `internal/infra/tools/builtin/larktools/docx_manage_test.go`
- multiple untracked reports under `docs/reports/`

`go list ./... | rg 'internal/infra/(lark|tools/builtin/larktools)'` showed both packages are present:
- `alex/internal/infra/lark`
- `alex/internal/infra/tools/builtin/larktools`

## Risk 1: docx create-doc convert chain
### Code path verified
- `internal/infra/tools/builtin/larktools/docx_manage.go`
  - `createDoc()` calls `client.Docx().CreateDocument(...)`
  - if initial content exists, it immediately calls `client.Docx().WriteMarkdown(ctx, doc.DocumentID, doc.DocumentID, content)`
- `internal/infra/lark/docx.go`
  - `ConvertMarkdownToBlocks(...)` calls `POST /docx/v1/documents/blocks/convert`
  - `WriteMarkdown(...)` is implemented in this package and consumes the convert result
- `internal/infra/tools/builtin/larktools/docx_manage_test.go`
  - shared test server explicitly mocks `/open-apis/docx/v1/documents/blocks/convert`
  - test coverage includes `TestDocxManage_CreateDoc_WithInitialContent` and `...UsesServerConvertMock`
- `internal/infra/lark/docx_test.go`
  - coverage includes `TestCreateDocument`, `TestConvertMarkdownToBlocks`, `TestUpdateDocumentBlockText`

### Real command evidence
```bash
go test -count=1 ./internal/infra/lark -run 'TestCreateDocument|TestConvertMarkdownToBlocks|TestUpdateDocumentBlockText|TestBuildDocumentURL' -v
go test -count=1 ./internal/infra/tools/builtin/larktools -run 'TestDocxManage_CreateDoc|TestDocxManage_ListBlocks|TestDocxManage_UpdateBlockText' -v
```
Result: all selected tests passed.

Conclusion: this chain is not just historical report residue; it is live in code and still has passing targeted coverage.

## Risk 2: historical larktools lint/backlog vs migration to internal/infra/lark
### What is true now
- `internal/infra/tools/builtin/larktools` still owns the user/tool adapter layer.
- `internal/infra/lark` owns typed Lark API wrappers and lower-level service logic.
- `larktools` imports `alex/internal/infra/lark` in active runtime files such as:
  - `docx_manage.go`
  - `drive_manage.go`
  - `wiki_manage.go`
  - `contact_manage.go`
  - `mail_manage.go`
  - `permission_grant.go`
  - `bitable_manage.go`
  - `okr_manage.go`
- Some task logic still remains directly in `larktools/task_manage.go`, including `listSubtasks` and `createSubtask`; this area is **not** migrated into `internal/infra/lark` yet.

### Real command evidence
```bash
golangci-lint run ./internal/infra/lark ./internal/infra/tools/builtin/larktools
```
Result: clean exit, no reported issues.

Additional grep evidence showed `createSubtask` / `listSubtasks` still defined in:
- `internal/infra/tools/builtin/larktools/task_manage.go`

Conclusion: the old "larktools lint backlog" is not supported by current targeted lint evidence, but the package itself absolutely still exists. The truthful state is **partial layering migration, not removal**.

## Suggested STATE.md updates
1. Add a fresh verification note that docx `create_doc` still routes through:
   - `larktools/docx_manage.go:createDoc()` -> `internal/infra/lark.DocxService.WriteMarkdown()` -> `/docx/v1/documents/blocks/convert`
   - with passing targeted tests in both packages.
2. Replace any wording implying `larktools` backlog is still assumed-open with:
   - targeted `golangci-lint run ./internal/infra/lark ./internal/infra/tools/builtin/larktools` is clean on current tree.
3. Clarify architecture wording to:
   - `internal/infra/lark` = typed SDK/service layer
   - `internal/infra/tools/builtin/larktools` = tool adapter layer still present, with some task logic still local (`listSubtasks`, `createSubtask`).
4. Keep one explicit remaining risk:
   - no live external Lark API contract verification was run in this cycle; evidence is unit/integration style local tests only.

## Commands run
```bash
pwd
git status --short --branch
find . -path '*lark*' | sort
go list ./... | rg 'internal/infra/(lark|tools/builtin/larktools)'
rg -n 'CreateDocument|ConvertMarkdownToBlocks|WriteMarkdown|blocks/convert|createSubtask|listSubtasks' internal docs/reports
read_file: STATE.md, internal/infra/tools/builtin/larktools/docx_manage.go, docx_manage_test.go, internal/infra/lark/docx.go, docx_test.go
go version
golangci-lint --version
go test -count=1 ./internal/infra/lark -run 'TestCreateDocument|TestConvertMarkdownToBlocks|TestUpdateDocumentBlockText|TestBuildDocumentURL' -v
go test -count=1 ./internal/infra/tools/builtin/larktools -run 'TestDocxManage_CreateDoc|TestDocxManage_ListBlocks|TestDocxManage_UpdateBlockText' -v
golangci-lint run ./internal/infra/lark ./internal/infra/tools/builtin/larktools
```
