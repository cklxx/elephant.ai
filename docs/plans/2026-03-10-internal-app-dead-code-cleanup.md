Status: completed

Objective: audit `internal/app/` for dead code and simplification opportunities, remove only clearly unused or duplicated logic, and keep behavior intact.

Steps:
1. Scan `internal/app/` for stale TODOs, dead exports, duplicate helpers, and unused code paths.
2. Validate each candidate by checking references and neighboring package patterns.
3. Apply the minimal cleanup, then run code review plus proportionate tests and push the result.

Outcome:
- Removed dead exported surface from `reminder` and `scheduler` where the symbols had no production callers.
- Moved blocker radar's test-only history inspection out of production code and into package tests.
- Cleaned the lingering `creds.XXX` placeholder comment in subscription selection logic.
- Fixed the existing `internal/app/context` prompt assertion drift so repo-wide tests pass again.
