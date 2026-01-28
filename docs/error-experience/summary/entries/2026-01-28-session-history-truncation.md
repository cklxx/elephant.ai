Summary: Session refresh dropped final summaries and subagent titles because persisted streaming events crowded out terminal events during replay.
Remediation: Drop output deltas/tool progress/streaming final chunks from history while keeping terminal finals and summaries; add regression coverage.
