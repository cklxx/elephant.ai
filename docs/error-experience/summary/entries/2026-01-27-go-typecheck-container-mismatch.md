Summary: Go validation failed due to unrelated Container API mismatches and signature changes in cmd/alex/internal packages.
Remediation: Align container fields/signatures or isolate those pending changes before running `make fmt vet test`.
