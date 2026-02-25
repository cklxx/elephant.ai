Summary: macOS `go test -race` emitted `malformed LC_DYSYMTAB` linker warnings with CLT-only toolchain.
Remediation: set `CGO_ENABLED=0` for tests on darwin (or install full Xcode). Dev script now defaults CGO off unless explicitly set.
