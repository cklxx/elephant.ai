# Streaming Stability and Doc Structure Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Stop streaming output from restarting on each delta update and keep document previews structured while still softening short final summaries.

**Architecture:** Use stable, identifier-based React keys for streaming events so React preserves component identity across timestamp updates. Add a lightweight markdown-structure heuristic in the final answer renderer to decide whether to apply summary-softening components or default markdown rendering.

**Tech Stack:** React, TypeScript, Vitest, Next.js.

### Task 1: Stabilize streaming keys for delta/final events

**Files:**
- Modify: `web/components/agent/ConversationEventStream.tsx`
- Test: `web/components/agent/__tests__/ConversationEventStream.test.tsx`

**Step 1: Write the failing test**

```tsx
it('keeps delta stream nodes stable across updates', () => {
  // existing test in ConversationEventStream.test.tsx
});
```

**Step 2: Run test to verify it fails**

Run: `npm test -- ConversationEventStream.test.tsx`
Expected: FAIL with assertion that the delta node was remounted.

**Step 3: Write minimal implementation**

```tsx
const key = getStableEventKey(event, index);
```

Implement `getStableEventKey` to return stable identifiers for
`workflow.node.output.delta` and `workflow.result.final`, and keep the existing
fallback for all other events.

**Step 4: Run test to verify it passes**

Run: `npm test -- ConversationEventStream.test.tsx`
Expected: PASS.

**Step 5: Commit**

```bash
git add web/components/agent/ConversationEventStream.tsx web/components/agent/__tests__/ConversationEventStream.test.tsx
git commit -m "fix: stabilize streaming event keys"
```

### Task 2: Preserve document structure in final answer rendering

**Files:**
- Modify: `web/components/agent/TaskCompleteCard.tsx`
- Test: `web/components/agent/__tests__/TaskCompleteCard.test.tsx`

**Step 1: Write the failing test**

```tsx
it('preserves headings and lists for document-like answers', () => {
  renderWithProvider({
    ...baseEvent,
    final_answer: '## Doc\n\n- Item 1\n- Item 2\n\n### Section\n\nMore text...',
    stop_reason: 'final_answer',
  });
  expect(document.querySelector('h2')).toBeInTheDocument();
  expect(document.querySelector('ul')).toBeInTheDocument();
});
```

**Step 2: Run test to verify it fails**

Run: `npm test -- TaskCompleteCard.test.tsx`
Expected: FAIL because headings/lists are currently softened into divs.

**Step 3: Write minimal implementation**

```tsx
const shouldSoftenSummary = !isDocumentLike;
```

Use a lightweight heuristic (length + heading/list counts) to decide when to apply
`summaryComponents`. If document-like, render with default markdown components.

**Step 4: Run test to verify it passes**

Run: `npm test -- TaskCompleteCard.test.tsx`
Expected: PASS.

**Step 5: Commit**

```bash
git add web/components/agent/TaskCompleteCard.tsx web/components/agent/__tests__/TaskCompleteCard.test.tsx
git commit -m "fix: keep document structure in final answers"
```

### Final validation

Run:
- `npm test`
- `npm run lint`
- `./scripts/run-golangci-lint.sh run ./...`
- `make test`

