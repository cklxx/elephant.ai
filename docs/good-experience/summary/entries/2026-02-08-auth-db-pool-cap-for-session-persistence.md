Summary: Added configurable auth DB pool cap with default `4` connections to prevent connection exhaustion from multi-instance dev processes.
Impact: Session refresh persistence is more stable across restarts and auth startup behavior is more predictable.
