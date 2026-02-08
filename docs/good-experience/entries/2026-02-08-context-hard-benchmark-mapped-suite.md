Practice: Map hard patterns from latest context-learning benchmarks into a dedicated offline suite collection rather than only scaling generic cases.

Why it worked:
- Bench-inspired patterns (lexical mismatch, sequential needles, multi-hop aggregation, long-context chunking) increased difficulty without introducing nondeterministic model judging.
- Collection-level isolation made it easy to observe whether harder cases degrade routing quality.
- The expanded suite remained availability-clean (`availability_errors=0`), so failures can be attributed to ranking semantics, not tool registration gaps.

Outcome:
- Suite expanded to 10 collections and 274 cases.
- Hard context-learning collection (`20/20`) now acts as a stable regression gate.
