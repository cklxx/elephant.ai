status: completed

## Scope

- Audit `web/` for unused TypeScript/React components.
- Audit `web/` for XSS or other content injection risks.
- Audit `web/` API callers for missing or inconsistent error handling.
- Apply only safe fixes, validate them, and commit a scoped change.

## Plan

1. Map component exports to actual imports/usages and confirm dead code candidates before deleting anything.
2. Trace all raw HTML/rendering boundaries and user-controlled URL/content paths for injection risk.
3. Review `web/lib/api*`, route handlers, and call sites for unhandled failures or inconsistent error normalization.
4. Implement low-risk fixes with focused tests/lint, run code review, then commit.

## Findings

- Confirmed one dead exported layout helper: `web/components/layout/page-shell.tsx` exported `ResponsiveGrid` but the symbol had no runtime references anywhere under `web/`.
- Multiple rendering paths accepted raw `href` values from markdown or json-render payloads without a local scheme allowlist, leaving `javascript:`-style link injection exposure in `DocumentCanvas`, inline task-complete markdown links, and json-render link widgets.
- `web/lib/attachment-text.ts`, `web/components/agent/ArtifactPreviewCard.tsx`, and `web/app/api/a2ui/preview/route.ts` had direct `fetch` calls with missing or incomplete failure normalization.
- No additional unused React component files were safely provable from static references alone without risking false positives from Next.js file-convention entrypoints and dynamic imports.

## Validation

- `npm ci` in `web/`
- `npm run lint`
- `npx vitest run --config vitest.config.mts lib/__tests__/safe-url.test.ts lib/__tests__/attachment-text.test.ts components/agent/__tests__/DocumentCanvas.attachments.test.tsx components/agent/__tests__/artifact-preview-html.test.tsx lib/__tests__/attachments.test.ts`
- `python3 skills/code-review/run.py review`
