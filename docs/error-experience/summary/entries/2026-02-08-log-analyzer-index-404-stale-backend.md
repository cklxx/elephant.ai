Summary: Log analyzer index endpoint returned 404 when a stale backend binary was still serving old routes and logs-ui reused the running process.
Remediation: Added logs-ui readiness probes with automatic backend/web restart and fail-fast diagnostics; added UI guidance for 404/401 states.
