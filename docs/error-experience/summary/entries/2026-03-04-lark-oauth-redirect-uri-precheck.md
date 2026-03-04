Summary: 生成 Lark OAuth 授权链接前，必须先校验 redirect_uri 白名单与网页能力配置；未预检会直接触发 20029 并阻断授权。

## Metadata
- id: errsum-2026-03-04-lark-oauth-redirect-uri-precheck
- tags: [summary, oauth, lark, redirect-uri, precheck]
- derived_from:
  - docs/error-experience/entries/2026-03-04-lark-oauth-redirect-uri-precheck.md
