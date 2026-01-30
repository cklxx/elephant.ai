Practice: Use a hook registry + per-request MemoryPolicy and skill cache/regex precompile to enable proactive memory/skills without channel-specific logic or hot-path regex churn.
Impact: Unified proactive injections across channels while keeping activation latency and token usage bounded.
