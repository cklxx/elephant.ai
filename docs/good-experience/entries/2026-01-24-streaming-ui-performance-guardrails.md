# 2026-01-24 - streaming UI performance guardrails

- Practice: Use event deduplication, LRU caps, RAF buffering, and defer markdown parsing until streaming settles for SSE UI rendering.
- Impact: Keeps the conversation UI responsive under high event volume and reduces render churn.
