# User Patterns

Updated: 2026-03-05 11:00

## Patterns

1. Governance and standards tasks must be specified at file granularity (file type -> directory -> naming -> blocking condition), not only directory-level abstractions.
2. Folder-governance outputs are expected to be deterministic and directly enforceable by review/check scripts.
3. `internal/**` rules must be explicit at first-level namespace and responsibility mapping level; high-level layer descriptions are insufficient.
4. Except unit tests, never use mock-based validation; inject/integration/live checks must run against real dependency paths.
5. For Feishu/Lark capabilities, user preference is CLI-first (`bash` + skills), and explicit `channel` tool registration should be removed rather than kept as compatibility wrapper.
6. If user asks to include current uncommitted changes in commit ("未提交的改动一起提交"), include both tracked and untracked workspace changes after a quick relevance/safety scan; do not exclude by default.
