# 2026-02-26 — Non-web defensive-bloat pruning and dead abstraction removal

Impact: Improved readability/maintainability by removing low-value defensive wrappers and dead helper APIs that increased cognitive load without adding safety.

## What changed

- Removed redundant `if len(...) > 0` guards before safe `for range` loops in multiple non-web paths (`preparation`, `hooks`, `config`, `lark delivery`, `llm`).
- Deleted unused abstractions with no production call-sites:
  - `StringArgStrict`, `BoolArgWithDefault`, `ContentSnippet`
  - `ToolAdapter.ValidateArguments` + its isolated test
  - `BuildAttachmentStoreMigrator` (deleted `attachment_uploader.go`)
- Inlined single-use helpers to reduce indirection:
  - `initEncoding` into `init`
  - `formatDurationShort` into `appendDurationSuffix`
  - `envelopeStreamFinished` into `isTerminalEvent`

## Why this worked

- Followed “minimal effective simplification”: only low-risk, behavior-preserving edits.
- Guard removal was limited to constructs already guaranteed safe by Go semantics (`range` over nil/empty maps/slices).
- Dead abstraction removal was backed by explicit zero-callsite scan before deletion.

## Validation

- `./scripts/go-with-toolchain.sh test ./internal/app/agent/preparation ./internal/app/agent/hooks ./internal/shared/config ./internal/delivery/channels/lark/testing ./internal/delivery/channels/lark ./internal/infra/skills ./internal/infra/llm ./internal/infra/tools/builtin/shared ./internal/infra/tools/builtin/artifacts ./internal/infra/mcp ./internal/shared/token ./internal/delivery/output ./internal/app/agent/coordinator ./cmd/alex`
- `./scripts/pre-push.sh`
- `python3 skills/code-review/run.py '{"action":"review"}'` + manual P0/P1 review

## Metadata
- id: good-2026-02-26-non-web-defensive-bloat-pruning
- tags: [good, maintainability, readability, defensive-programming, dead-code]
- links:
  - docs/plans/2026-02-26-non-web-systematic-maintainability-optimization.md
  - internal/shared/config/runtime_file_loader.go
  - internal/infra/tools/builtin/shared/helpers.go
  - internal/infra/mcp/tool_adapter.go
