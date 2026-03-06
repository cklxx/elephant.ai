# larktools docx manage fix report (2026-03-06)

## Scope
- Target: `internal/infra/tools/builtin/larktools/docx_manage_test.go`
- Goal: ensure `/open-apis/docx/v1/documents/blocks/convert` mock path + reasonable convert payload are covered in `TestDocxManage_CreateDoc_WithInitialContent`.

## Changes made
- Updated `TestDocxManage_CreateDoc_WithInitialContent`:
  - assert create-doc API was called (`createCalled`)
  - assert convert payload contains `"content_type":"markdown"`
  - keep convert-route assertion and descendant insertion assertion
- No production code changes.

## Commands & results
- `go test -count=1 ./internal/infra/tools/builtin/larktools/...` → **PASS**
- `golangci-lint run ./internal/infra/tools/builtin/larktools/...` → **PASS**

## Changed files
- `internal/infra/tools/builtin/larktools/docx_manage_test.go`

## Remaining issues
- None found in the validated scope (`larktools/...`).
- Note: repository has other pre-existing unrelated modified/untracked files outside this task.

