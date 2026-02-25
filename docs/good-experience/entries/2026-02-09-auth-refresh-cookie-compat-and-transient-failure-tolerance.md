Practice: Keep refresh login persistence stable by combining backend cookie encoding compatibility with frontend refresh error classification.

Why it worked:
- Backend writes URL-safe token cookies and accepts both legacy standard base64 and URL-safe cookie values, reducing decode mismatch risk across environments.
- Frontend only clears local auth session on definitive auth errors (400/401/403), so transient 5xx/network failures do not force unnecessary re-login.
- Group-chat model selection now ignores legacy chat+user fallback to avoid sender-specific stale overrides in shared chats.

Outcome:
- Reload-after-login stability improved for expired-access-token paths where refresh must run.
- Group-chat model selection resolution is consistently chat-scoped, reducing `apikey not registered` incidents caused by legacy sender-level drift.
- Added regression tests in Go and web auth client to guard against recurrence.
