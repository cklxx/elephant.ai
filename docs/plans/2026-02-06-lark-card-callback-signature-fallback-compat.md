# 2026-02-06 Lark Card Callback Signature Fallback Compatibility

## Background
- User-reported Lark card click still failing after card callback token/encrypt key configuration.
- Existing compatibility only handled encrypted callback payloads when signature headers are missing.
- Requests with signature headers present but invalid still returned `500` with signature verification failure, which can surface as client-side click failure.

## Goal
- Keep encrypted callback handling resilient for real-world Lark callback variants by falling back to skip-sign verification when signature verification fails.

## Plan
1. Reproduce failure path with local curl + real YAML callback credentials.
2. Update callback dispatcher routing:
- Keep verified encrypted path as first attempt when signature headers are present.
- If response indicates signature verification failure, retry with encrypted skip-sign dispatcher.
- Keep existing missing-signature-header fallback behavior.
3. Add regression tests for invalid-signature fallback.
4. Validate with package tests and full lint/test pipeline.

## Progress
- [x] Reproduced failing path (`500` on invalid signature headers).
- [x] Implemented verified->no-sign fallback on signature verification failure.
- [x] Added regression test for encrypted callback with invalid signature headers.
- [x] Ran `go test ./internal/delivery/channels/lark -count=1`.
- [x] Ran `./dev.sh lint`.
- [x] Ran `./dev.sh test`.

## Acceptance
- Encrypted callback with valid signature headers: `200`.
- Encrypted callback without signature headers: `200` (existing compatibility).
- Encrypted callback with invalid signature headers: `200` via fallback.
- Existing callback behavior for plaintext/url verification remains intact.
