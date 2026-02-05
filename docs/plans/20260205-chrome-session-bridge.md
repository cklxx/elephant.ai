# Plan: Chrome Session Bridge (XHS cookies via extension)

Owner: cklxx (requester) / implementation by Codex agent  
Date: 2026-02-05  
Branch: `eli/chrome-session-bridge` (worktree)

## Goal
Allow `alex` (with `runtime.toolset: local`) to reuse the **currently running** macOS Google Chrome session (default profile, existing login state) by communicating with a **Manifest V3 Chrome extension** over a localhost WebSocket bridge.

MVP delivers:
- `browser_session_status`: show extension connection + tab list
- `browser_cookies`: return `Cookie` header / structured cookies for a domain
- `browser_storage_local` (optional): read localStorage keys from a tab

Non-goals for MVP:
- Full DOM automation via `chrome.debugger` / `cdp.send` (planned follow-up)
- Any bypass of site risk controls (XHS 461, CAPTCHAs, etc.)

## Milestones
1) Config wiring ✅
2) Bridge server (WS + JSON-RPC client) ✅
3) Tools: status/cookies/storage ✅
4) Chrome extension (MV3) + install docs ✅
5) Full lint/test ⏳

## Progress log
- 2026-02-05: Plan created.
- 2026-02-05: Added `runtime.browser.connector` + bridge config; implemented WS bridge + MVP tools; added MV3 extension + ops doc.

## Acceptance criteria
- Extension can connect to `ws://127.0.0.1:17333/ws` and receives `welcome`.
- `browser_session_status` reports connected + returns tabs.
- With XHS logged in on Chrome, `browser_cookies(domain=xiaohongshu.com)` returns non-empty cookie header.
- Restarting Chrome reconnects and tools continue to work.
