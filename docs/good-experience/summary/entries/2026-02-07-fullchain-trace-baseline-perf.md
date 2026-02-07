Summary: Full-chain trace instrumentation is most reliable when anchored in ReAct runtime boundaries and correlated via run/session attributes across HTTP, task, and SSE paths.
Impact: Faster bottleneck localization plus measurable baseline performance gains from lower stream event volume and cheaper attachment dedupe.
