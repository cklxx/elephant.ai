Practice: Introduce a dedicated memory-capabilities collection (memory_search/memory_get + memory-informed execution chains) rather than embedding memory checks sparsely in mixed collections.

Why it worked:
- Memory-specific cases increased signal for retrieval/read regressions that were previously diluted.
- Memory chain cases (search -> get -> execute/patch/report) validated real workflow routing, not isolated tool picks.
- The new layer scaled without availability regressions, so semantic drift remains easy to diagnose.

Outcome:
- Foundation suite expanded to `11/11` collections and `294/294` cases.
- Memory capabilities are now a first-class regression gate with `20/20` pass visibility.
