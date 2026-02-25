# Antigravity 403 Model List Investigation Plan

## Context
- Investigate 403 Forbidden on Antigravity model list for IDE-sourced credentials.
- Base URL: https://cloudcode-pa.googleapis.com
- Source: antigravity_ide (from ~/.gemini/oauth_creds.json)

## Preflight
- claude -p: failed (rate limit reached; resets 2026-01-31 14:00 Asia/Shanghai). Proceeding with manual plan.

## Plan
1. Inspect ~/.gemini for credential/config sources; summarize relevant fields (expiry, scopes, client metadata) without secrets.
2. Trace model list request path in repo (runtime vs subscription catalog) and compare with Antigravity/Gemini expectations.
3. Identify request deltas between IDE path and Antigravity client (method, endpoint, headers/body).
4. Propose fixes and optional patch plan; confirm with cklxx before coding.

## Progress Log
- 2026-01-31 13:00: Plan created.
- 2026-01-31 13:05: Reviewed ~/.gemini settings + oauth creds (oauth-personal; token valid until 2026-01-31 13:23:35 +08:00; scopes include cloud-platform).
- 2026-01-31 13:10: Traced model list flow: UI uses subscription catalog -> POST /v1internal:fetchAvailableModels (antigravity).
- 2026-01-31 13:15: Reproduced 403 via direct POST; adding x-goog-user-project yields PERMISSION_DENIED with USER_PROJECT_DENIED details.
- 2026-01-31 13:20: Probed request schema: fetchAvailableModels accepts `project` field; unknown fields (requestId/userAgent/requestType) are rejected with 400.
- 2026-01-31 13:40: Reviewed public proxy repos + official Gemini CLI/Code Assist docs; proxies typically wrap Gemini CLI OAuth or Gemini API key, and Code Assist requires project + Service Usage Consumer access.
- 2026-01-31 13:50: Located Antigravity IDE logs under `~/Library/Application Support/Antigravity/logs/*/window*/exthost/google.antigravity/Antigravity.log`.
- 2026-01-31 13:50: Logs show IDE uses `https://daily-cloudcode-pa.googleapis.com` and logs URLs for `v1internal:streamGenerateContent`, but not `fetchAvailableModels` at info level.
- 2026-01-31 13:55: Verified built-in Antigravity extension bundled at `/Applications/Antigravity.app/Contents/Resources/app/extensions/antigravity` with `language_server_macos_arm` binary (likely HTTP client; respects standard HTTP(S)_PROXY).
