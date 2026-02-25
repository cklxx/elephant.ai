Practice: Add trace spans directly on ReAct iteration, LLM streaming, and tool execution hot paths with shared run/session attributes, then pair with low-risk hot-path optimizations (delta batching + lightweight attachment signatures).
Impact: Enables end-to-end latency attribution for performance analysis while reducing SSE/event serialization overhead without changing product behavior.
