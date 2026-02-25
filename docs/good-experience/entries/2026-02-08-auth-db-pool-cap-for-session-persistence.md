Practice: For auth/session critical stores in development, set conservative DB pool defaults and expose explicit pool-cap config to avoid hidden fallback behaviors under multi-process load.
Impact: Reduced probability of `too many clients` causing memory-store fallback, so refresh sessions remain durable across restarts in normal dev workflows.
