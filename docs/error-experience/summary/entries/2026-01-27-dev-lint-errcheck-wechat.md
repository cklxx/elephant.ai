# 2026-01-27 - dev.sh lint blocked by unchecked bot.Block error

- Summary: `./dev.sh lint` failed because `internal/channels/wechat/gateway.go:76` ignores the error from `bot.Block()`.
- Remediation: handle or explicitly discard the error to satisfy `errcheck`.
- Resolution: not fixed in this run (outside logging scope).
