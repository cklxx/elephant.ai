Practice: Expand foundation evaluation with new thematic collections instead of only enlarging existing files, then close all new bad cases by tightening intent wording where lexical conflicts dominate ranking.

Why it worked:
- New collections (`availability_recovery`, `valuable_delivery_workflows`) increased coverage breadth without changing runtime code paths.
- First-pass failures were isolated to one collection and quickly diagnosable from case-level `failure_type`/`hit_rank`.
- Small intent text refinements resolved ranking collisions while preserving expected tool semantics.

Outcome:
- Foundation suite moved from 4 to 6 collections.
- Total cases scaled to 190 and stabilized at `190/190` with `availability_errors=0`.
- Documentation now includes explicit per-collection and total `x/x` counters for fast regression tracking.
