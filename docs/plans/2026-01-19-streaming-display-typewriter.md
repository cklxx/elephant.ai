# Streaming Display Typewriter Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Keep backend streaming buffered while the web UI and CLI render per-character scrolling during streaming, with full Markdown rendering on completion.

**Architecture:** Leave SSE and agent event chunking unchanged. Add a display-layer typewriter that reveals characters one at a time. On the web, split streamed content into a stable Markdown prefix and a raw tail to avoid partial markdown parsing. On the CLI, emit per-character output only for non-final streaming chunks to preserve Markdown line rendering.

**Tech Stack:** Go (CLI), Next.js/React (web), Vitest.

### Task 1: Web streaming split helper (test first)

**Files:**
- Create: `web/components/agent/__tests__/streamingMarkdownRenderer.test.tsx`

**Step 1: Write the failing test**

```ts
import { describe, expect, it } from "vitest";
import { splitStreamingContent } from "@/components/agent/StreamingMarkdownRenderer";

const longText = "a".repeat(200);

describe("splitStreamingContent", () => {
  it("keeps a raw tail for long streamed content", () => {
    const result = splitStreamingContent(longText, longText.length);
    expect(result.stable.length).toBeLessThan(longText.length);
    expect(result.tail.length).toBeGreaterThan(0);
    expect(result.stable + result.tail).toBe(longText);
  });
});
```

**Step 2: Run test to verify it fails**

Run: `npm test -- streamingMarkdownRenderer` (from `web/`)

Expected: FAIL because `splitStreamingContent` does not exist.

### Task 2: Web typewriter rendering (minimal implementation)

**Files:**
- Modify: `web/components/agent/StreamingMarkdownRenderer.tsx`

**Step 1: Implement the helper and typewriter target length**

```ts
export function splitStreamingContent(content: string, visibleLength: number) {
  const visible = content.slice(0, Math.max(0, visibleLength));
  const safeLength = findSafeRenderLength(visible);
  return {
    stable: visible.slice(0, safeLength),
    tail: visible.slice(safeLength),
  };
}
```

- Set `targetLength` to the full `normalizedContent.length` when streaming.
- Initialize `displayedLength` to 0 when streaming (so the first character appears via the animation tick).
- Keep the typewriter step at 1 character per tick (set `TYPEWRITER_MAX_STEP = 1`).
- Render `stable` through `LazyMarkdownRenderer` and `tail` as raw text (`whitespace-pre-wrap`) only while streaming. When `streamFinished`, render full Markdown as today.

**Step 2: Run test to verify it passes**

Run: `npm test -- streamingMarkdownRenderer` (from `web/`)

Expected: PASS.

### Task 3: CLI typewriter helper (test first)

**Files:**
- Create: `cmd/alex/typewriter_test.go`

**Step 1: Write the failing test**

```go
package main

import "testing"

func TestEmitTypewriterPreservesRunes(t *testing.T) {
	var out string
	emitTypewriter("hi好", func(s string) {
		out += s
	})
	if out != "hi好" {
		t.Fatalf("expected output to preserve runes, got %q", out)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./cmd/alex -run TestEmitTypewriterPreservesRunes -v`

Expected: FAIL because `emitTypewriter` does not exist.

### Task 4: CLI typewriter implementation + wiring

**Files:**
- Create: `cmd/alex/typewriter.go`
- Modify: `cmd/alex/stream_output.go`
- Modify: `cmd/alex/tui_bubbletea.go`

**Step 1: Implement helper**

```go
package main

func emitTypewriter(text string, emit func(string)) {
	for _, r := range text {
		emit(string(r))
	}
}
```

**Step 2: Wire in streaming output handlers**
- In `cmd/alex/stream_output.go`, for streaming chunks where `chunk.completeLine` is false, call `emitTypewriter(rendered, h.write)` instead of `h.write(rendered)`.
- In `cmd/alex/tui_bubbletea.go`, for streaming chunks where `chunk.completeLine` is false, call `emitTypewriter(rendered, m.appendAgentRaw)`.
- Keep full-line markdown rendering as-is to avoid splitting ANSI escape sequences.

**Step 3: Run test to verify it passes**

Run: `go test ./cmd/alex -run TestEmitTypewriterPreservesRunes -v`

Expected: PASS.

### Task 5: Full validation

**Step 1: Go tests**

Run: `go test ./...`

Expected: PASS.

**Step 2: Web lint + tests**

Run: `npm run lint` (from `web/`)

Run: `npm test` (from `web/`)

Expected: PASS.

### Task 6: Commit

```bash
git add web/components/agent/StreamingMarkdownRenderer.tsx \
  web/components/agent/__tests__/streamingMarkdownRenderer.test.tsx \
  cmd/alex/typewriter.go cmd/alex/typewriter_test.go \
  cmd/alex/stream_output.go cmd/alex/tui_bubbletea.go

git commit -m "feat: typewriter streaming display in web and CLI"
```
