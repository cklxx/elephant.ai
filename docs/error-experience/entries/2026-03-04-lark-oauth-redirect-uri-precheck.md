# 2026-03-04 · Lark OAuth 回调地址未预检导致 20029

## Context
- 用户要求“走 OAuth”完成 `user_access_token` 授权。
- 授权页返回 `20029 redirect_uri 请求不合法`，并提供 log ID。

## Symptom
- OAuth 无法进入授权确认页，流程在入口即失败。

## Root Cause
- 生成授权链接前，未先验证 `redirect_uri` 是否已在飞书开放平台安全设置白名单中。
- 未先确认应用是否启用网页能力并完成对应回调配置。

## Remediation
- 任何 Lark OAuth 链路必须先做三项预检，再输出授权链接：
  - 预检 1：`redirect_uri` 与后台“安全设置 > 重定向 URL”逐字符一致（含协议、域名、端口、路径）。
  - 预检 2：应用已启用网页能力，并完成网页相关配置。
  - 预检 3：若本地回调不可用，优先改用已备案的公网 HTTPS 回调地址；否则先建隧道并加入白名单后再发起。
- 预检失败时不再引导用户点击链接，先给出配置修复步骤。

## Follow-up
- 将 OAuth 链路“先预检后发起”作为默认执行顺序，避免再次触发 20029。

## Metadata
- id: err-2026-03-04-lark-oauth-redirect-uri-precheck
- tags: [oauth, lark, redirect-uri, user-correction, precheck]
- links:
  - docs/error-experience/summary/entries/2026-03-04-lark-oauth-redirect-uri-precheck.md
  - docs/plans/2026-03-04-lark-task-tenant-token-and-visibility.md
