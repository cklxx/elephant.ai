Summary: OAuth login endpoint returned 503 because auth config lacked env fallback and auth DB failures disabled the entire module.
Remediation: Added auth env fallback in bootstrap config and development-mode fallback to memory stores when JWT/DB is unavailable, plus server e2e route tests.
