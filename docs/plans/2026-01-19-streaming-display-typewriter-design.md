# Streaming Display Typewriter Design

## Goal
Keep backend streaming buffered while presenting per-character scrolling in web and CLI clients. Final output remains fully rendered Markdown.

## Non-goals
- Change server-side streaming payloads or event types.
- Modify provider streaming chunk sizes.
- Alter response schemas or transport protocols.

## Current State
The agent emits `workflow.node.output.delta` and `workflow.result.final` events in buffered chunks (e.g., 256+ chars, 800+ chars). Web and CLI render these chunks directly, so users perceive chunked updates rather than per-character typing. The web already has a streaming markdown renderer with a typewriter effect, but it advances in multi-character steps and intentionally waits for safe markdown boundaries. CLI buffers streaming markdown and flushes on lines or thresholds.

## Proposed Design
### Backend
No changes. Keep buffered streaming for bandwidth and event volume stability.

### Web
Implement per-character rendering at display time while preserving final Markdown correctness. The streaming renderer will:
- Maintain a target length that tracks the full streamed content length.
- Increment displayed length by one character per animation tick (bounded by frame rate).
- During streaming, allow temporary partial Markdown rendering (or render the unstable tail as plain text) to avoid blocking on safe boundaries.
- When `streamFinished` is true, render the full content normally.

This keeps transport buffered while the UI shows character-by-character progress.

### CLI
Keep current buffered parsing for markdown safety, but render each emitted chunk using a per-character writer. The typewriter effect happens after markdown chunk rendering, not at the transport layer. Final output remains rendered in the existing completion step.

## Data Flow
1. LLM streams buffered deltas to the agent.
2. SSE forwards buffered deltas unchanged.
3. Web and CLI render chunks with a per-character typewriter effect.
4. When streaming ends, render the final Markdown output in full.

## Risks and Mitigations
- Partial Markdown during streaming may look odd. Mitigate by re-rendering full Markdown on completion.
- Per-character rendering can increase CPU usage. Mitigate by using requestAnimationFrame and minimal state updates.

## Testing
- Web: unit test that streaming updates render progressively and converge to full content.
- CLI: unit/integration test that per-character writer preserves content order and final output.
