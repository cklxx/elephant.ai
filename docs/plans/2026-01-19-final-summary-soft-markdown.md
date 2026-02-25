# Final Summary Soft-Structured Rendering Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make the final answer render as concise paragraph-style text with minimal visible Markdown structure, while keeping emphasis and attachment rendering intact.

**Architecture:** Keep event payloads unchanged and adjust only the final summary rendering path. `TaskCompleteCard` will pass a dedicated component override set to `AgentMarkdown` to soften headings and lists. Update the default persona voice to nudge the model toward short paragraph summaries with minimal structure, especially when attachments are present.

**Tech Stack:** React/Next.js (web), Tailwind CSS, Vitest, YAML context config.

### Task 1: Add failing UI test for softened summary rendering

**Files:**
- Modify: `web/components/agent/__tests__/TaskCompleteCard.test.tsx`

**Step 1: Write the failing test**

```tsx
it("renders final answer without heading/list elements", () => {
  const { container } = renderWithProvider({
    ...baseEvent,
    final_answer: "# Summary\n\n- First\n- Second\n\n**Key:** Detail.",
    stop_reason: "final_answer",
  });

  expect(screen.getByText(/Summary/i)).toBeInTheDocument();
  expect(screen.getByText(/First/i)).toBeInTheDocument();
  expect(container.querySelector("h1")).toBeNull();
  expect(container.querySelector("ul")).toBeNull();
  expect(container.querySelector("ol")).toBeNull();
  expect(container.querySelector("strong")).toBeInTheDocument();
});
```

**Step 2: Run test to verify it fails**

Run: `npm test -- TaskCompleteCard`
Expected: FAIL because `h1`/`ul`/`ol` elements are still rendered.

### Task 2: Implement softened summary rendering in TaskCompleteCard

**Files:**
- Modify: `web/components/agent/TaskCompleteCard.tsx`

**Step 1: Write minimal implementation**

Add a `summaryComponents` object passed to `AgentMarkdown` that overrides:
- `h1`-`h6` to render as `<div>` with modest emphasis.
- `ul`/`ol` to render as `<div>` containers with vertical spacing and no bullets.
- `li` to render as `<div>` paragraph blocks.
- `blockquote`/`hr` to render as simple paragraph separators.
- `strong` to use slightly stronger emphasis (e.g. `font-semibold`).
- Preserve existing `img` override for inline media.

Example snippet:

```tsx
const summaryComponents = {
  h1: (props) => <div className="mt-2 font-medium text-foreground" {...props} />,
  ul: (props) => <div className="my-2 space-y-1" {...props} />,
  li: (props) => <div className="whitespace-pre-wrap" {...props} />,
  strong: (props) => <strong className="font-semibold text-foreground" {...props} />,
  // img override kept as-is
};
```

**Step 2: Run test to verify it passes**

Run: `npm test -- TaskCompleteCard`
Expected: PASS.

### Task 3: Update default persona guidance (Option C)

**Files:**
- Modify: `configs/context/personas/default.yaml`

**Step 1: Update persona voice**

Add guidance to keep final summaries paragraph-based with minimal headings/lists and to be brief when documents are attached.

**Step 2: Validate formatting**

Run: `rg -n "summary" configs/context/personas/default.yaml` to confirm changes are in place.

### Task 4: Run full verification

**Files:**
- None

**Step 1: Frontend tests**

Run: `npm test`
Expected: PASS

**Step 2: Frontend lint**

Run: `npm run lint`
Expected: PASS

**Step 3: Go lint**

Run: `./scripts/run-golangci-lint.sh run ./...`
Expected: PASS

**Step 4: Go tests**

Run: `make test`
Expected: PASS

### Task 5: Commit and push

**Step 1: Commit**

```bash
git add web/components/agent/__tests__/TaskCompleteCard.test.tsx \
  web/components/agent/TaskCompleteCard.tsx \
  configs/context/personas/default.yaml \
  docs/plans/2026-01-19-final-summary-soft-markdown-design.md \
  docs/plans/2026-01-19-final-summary-soft-markdown.md \
  internal/server/app/postgres_event_history_store.go \
  internal/server/app/postgres_event_history_store_test.go \
  internal/server/bootstrap/server.go

git commit -m "fix: persist history attachments and soften final summaries"
```

**Step 2: Push**

Run: `git push`
