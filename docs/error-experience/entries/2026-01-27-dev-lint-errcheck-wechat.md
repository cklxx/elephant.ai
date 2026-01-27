# 2026-01-27 - dev.sh lint blocked by unchecked bot.Block error

## Error
- `./dev.sh lint` failed with `errcheck` in `internal/channels/wechat/gateway.go:76` (unchecked `bot.Block()` error).

## Impact
- Full lint step could not complete.

## Notes / Suspected Causes
- Unrelated to log standardization; pre-existing lint issue in the wechat gateway.

## Remediation Ideas
- Handle or intentionally ignore the error from `bot.Block()` (e.g., assign and log).

## Resolution (This Run)
- Not resolved; left unchanged to keep scope focused on logging changes.
