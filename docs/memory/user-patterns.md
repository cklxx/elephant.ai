# User Patterns

Updated: 2026-03-04 12:00

## Patterns

1. Governance and standards tasks must be specified at file granularity (file type -> directory -> naming -> blocking condition), not only directory-level abstractions.
2. Folder-governance outputs are expected to be deterministic and directly enforceable by review/check scripts.
3. `internal/**` rules must be explicit at first-level namespace and responsibility mapping level; high-level layer descriptions are insufficient.
4. Except unit tests, never use mock-based validation; inject/integration/live checks must run against real dependency paths.
