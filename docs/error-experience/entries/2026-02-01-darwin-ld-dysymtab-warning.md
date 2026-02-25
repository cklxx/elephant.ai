# 2026-02-01 - Darwin ld LC_DYSYMTAB warnings during go test

## Error
- `go test -race` emitted warnings like: `ld: warning: ... malformed LC_DYSYMTAB` on macOS (CLT-only toolchain).

## Impact
- Noisy test output; tests still pass but warnings reduce signal.

## Notes
- Observed with Command Line Tools only (no full Xcode).

## Remediation
- Disable cgo during tests on macOS (`CGO_ENABLED=0`), or install full Xcode toolchain.

## Status
- mitigated (dev.sh defaults CGO off for darwin tests).
