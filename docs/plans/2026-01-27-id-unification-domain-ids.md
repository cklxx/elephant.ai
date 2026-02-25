# Plan: ID unification + domain layering doc (2026-01-27)

## Goal
- Unify ID usage within a reasonable scope by centralizing log_id in OutputContext and reducing parameter sprawl.
- Document domain layering and ID semantics in a dedicated reference file.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Add LogID to OutputContext and propagate from context IDs.
2. Simplify workflow event bridge wiring to consume OutputContext directly.
3. Write a domain layering + ID semantics reference doc with YAML examples.
4. Run full lint + tests.
5. Commit changes (small, focused commits).

## Progress
- 2026-01-27: Plan created; engineering practices reviewed.
- 2026-01-27: Added LogID to OutputContext, simplified workflow event bridge wiring, and drafted domain layering + ID semantics doc.
- 2026-01-27: Ran `./dev.sh lint` and `./dev.sh test` (pass; LC_DYSYMTAB linker warnings emitted).
