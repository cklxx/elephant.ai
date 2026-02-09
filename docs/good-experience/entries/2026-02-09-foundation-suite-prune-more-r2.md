# 2026-02-09 — Foundation Suite Further Prune (R2)

## Context
After reducing to 499, the suite still looked oversized. We needed another review-driven reduction while preserving diagnostic utility.

## What Changed
- Applied second-round per-collection caps with all 25 dimensions retained.
- Retired 54 additional low-signal/redundant cases.
- Kept all three hard stress collections unchanged at 16 each.

## Impact
- Case volume: `499 -> 445`.
- pass@5 remained full: `445/445`.
- pass@1: `375/445` (84.3%), still preserving optimization pressure.
- Deliverable: `23/28` good.

## Learnings
- A two-stage prune (first under-500, then review-based squeeze) is effective for eliminating residual redundancy.
- Keeping hard stress collections fixed while shrinking general collections preserves challenge diversity.
