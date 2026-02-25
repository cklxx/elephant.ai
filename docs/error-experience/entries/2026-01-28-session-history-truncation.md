Error: Session refresh dropped final summaries and subagent titles because high-volume streaming events were persisted and replay missed the terminal completion.
Remediation: Skip persisting output deltas, tool progress, and streaming final chunks while keeping terminal finals and summaries; add regression tests.
