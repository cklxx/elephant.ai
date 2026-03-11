# Web TypeScript React Audit

Date: 2026-03-11
Branch: `audit/web-typescript-react`

## Scope

Audit `web/` TypeScript/React code and fix:
- unused imports
- `any` misuse
- list rendering missing `key` props

## Plan

1. Run targeted static analysis in `web/` to identify concrete violations.
2. Fix only the relevant files with minimal, typed changes.
3. Run focused lint/tests, then code review, commit, and fast-forward merge back to `main`.

## Progress

- [x] Static analysis completed
- [x] Fixes implemented
- [x] Validation completed
- [ ] Merged back to `main`

## Findings

- Removed compiler-confirmed unused imports from targeted debug, SSE, and agent card files.
- Replaced explicit `any` in the visualizer SSE route, the stop-spinner test, and tool panel rendering with typed `unknown` or concrete payload types.
- Audited JSX list rendering with `react/jsx-key`; no missing `key` prop violations remained in the checked surface, so the fix focused on improving unstable index-based keys in shared highlight rendering and normalizing tool panel children through `Children.toArray`.

## Validation

- `node /Users/bytedance/code/elephant.ai/web/node_modules/typescript/bin/tsc --noEmit --pretty false`
- `node /Users/bytedance/code/elephant.ai/web/node_modules/eslint/bin/eslint.js app/api/visualizer/stream/route.ts app/__tests__/conversation-stop-spinner.test.tsx components/agent/tooling/ToolPanels.tsx components/agent/ToolCallCard.tsx components/dev-tools/shared/highlight-text.tsx --rule '@typescript-eslint/no-explicit-any:error' --rule 'react/jsx-key:error'`
- `node /Users/bytedance/code/elephant.ai/web/node_modules/vitest/vitest.mjs run app/__tests__/conversation-stop-spinner.test.tsx`
