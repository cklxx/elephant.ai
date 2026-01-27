# 2026-01-24 - backend resource guardrails

- Practice: Precompile hot-path regex, cap response reads with size limits, and apply retention/backpressure for event history storage.
- Impact: Reduces allocation pressure and prevents unbounded growth during long sessions.
