# ALEX Integration Tests

Updated: 2026-03-10

## Directory Structure

```
tests/
├── README.md                          # This file
├── Makefile                           # Test runner targets
├── acceptance/                        # Deprecated (shell chain removed 2026-02-12)
│   └── README.md
├── config/
│   └── acceptance-criteria.yml        # Acceptance criteria definitions
├── scenarios/
│   └── lark_http/                     # Lark HTTP endpoint test scenarios (YAML)
└── scripts/
    └── run-integration.sh             # Integration test runner
```

## Running Tests

```bash
# Run integration tests via the shell runner
make test-integration

# Run a quick subset
make test-quick
```

## Scenario Files

`scenarios/lark_http/` contains YAML-based test scenarios for Lark HTTP endpoints.
Each file defines request/response pairs and assertions for a specific feature.

## Notes

- The legacy shell acceptance chain was removed on 2026-02-12 (see `acceptance/README.md`).
- Unit and package-level tests live alongside source code under `internal/`.
- E2E evaluation tests live under `evaluation/`.
