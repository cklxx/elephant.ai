# Plan: Replace legacy reference doc with AGENTS.md + adjust CLI config/init (2026-01-27)

## Goal
Remove the legacy reference doc, standardize docs to reference `AGENTS.md`, and align CLI usage/config initialization messaging with the canonical docs and resolved config path.

## Constraints
- Keep config examples YAML-only.
- Update plan progress as work proceeds.
- Run full lint + tests before delivery.

## Plan
1. **Inventory + scope**
   - Identify references to the legacy reference doc and any stale CLI documentation pointers.
   - Confirm current CLI config/init entry points and config path resolution.

2. **Docs consolidation**
   - Remove the legacy reference doc file.
   - Update references in README, docs landing pages, Makefile docs target, and architecture flow notes to point to `AGENTS.md` (and other relevant docs).

3. **CLI config/init adjustments**
   - Update CLI usage output to reference AGENT/architecture flow docs instead of the removed ALEX doc.
   - Show the resolved config path (respecting `ALEX_CONFIG_PATH`) in CLI usage/help to match actual config resolution.
   - Add/adjust tests for any new CLI helper logic.

4. **Validation + wrap-up**
   - Run `./dev.sh lint` and `./dev.sh test`.
   - Commit changes in small, logical commits.

## Progress
- [x] Step 1 complete
- [x] Step 2 complete
- [x] Step 3 complete
- [x] Step 4 complete
