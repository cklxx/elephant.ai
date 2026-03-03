# 2026-03-03 Lark emoji-only reply fix

## Background
Recent logs show Lark receives user messages and adds emoji reactions, but some replies never arrive.
Observed hard failure in `logs/alex-service.log`:
- `Lark dispatch message failed: ... code=230001 ... message_content_text_tag's text field can't be nil`

This indicates malformed `post` payload construction and no graceful fallback path.

## Plan
1. Fix post payload encoding so `text` tag always carries a `text` field (including blank lines).
2. Add delivery fallback: if post delivery is rejected for invalid content, retry once as plain text.
3. Add regression tests for both payload correctness and fallback delivery behavior.
4. Run targeted tests for `internal/delivery/channels/lark`.

## Progress
- [x] Root-cause from logs identified (`230001` invalid post payload)
- [x] Code fix implemented
- [x] Regression tests added
- [x] Verification complete
