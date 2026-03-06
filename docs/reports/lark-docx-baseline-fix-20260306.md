# Lark docx test baseline fix report

- Time: 2026-03-06
- Target: `go test -count=1 ./internal/infra/tools/builtin/larktools/...`
- Focus scenario: `TestDocxManage_CreateDoc_WithInitialContent`

## Completed work
- Confirmed real docx test path remains `internal/infra/tools/builtin/larktools`.
- Verified mock server already includes `/open-apis/docx/v1/documents/blocks/convert` interception via shared helper in `docx_manage_test.go`.
- Hardened regression guard in `TestLarkTestServerWithDocxConvertMock_HandlesConvertRoutes` by asserting the convert response contains:
  - `first_level_block_ids`
  - converted `block_id`
  - `text_run.content` payload
  - `parent_id`

## Files changed
- `internal/infra/tools/builtin/larktools/docx_manage_test.go`

## Commands run
```bash
rg -n "TestDocxManage_CreateDoc_WithInitialContent|docx_manage_test|/open-apis/docx/v1/documents/blocks/convert" /Users/bytedance/code/elephant.ai
go test -count=1 ./internal/infra/tools/builtin/larktools/...
go test -count=1 -run TestDocxManage_CreateDoc_WithInitialContent ./internal/infra/tools/builtin/larktools
gofmt -w internal/infra/tools/builtin/larktools/docx_manage_test.go
go test -count=1 -run 'TestDocxManage_CreateDoc_WithInitialContent|TestLarkTestServerWithDocxConvertMock_HandlesConvertRoutes' ./internal/infra/tools/builtin/larktools
go test -count=1 ./internal/infra/tools/builtin/larktools/...
```

## Results
- `go test -count=1 ./internal/infra/tools/builtin/larktools/...` ✅ PASS
- `go test -count=1 -run TestDocxManage_CreateDoc_WithInitialContent ./internal/infra/tools/builtin/larktools` ✅ PASS
- Convert mock regression guard now checks payload shape, not just status code/path.

## Remaining risk
- Other unrelated working tree changes exist outside this file; not touched in this cycle.

