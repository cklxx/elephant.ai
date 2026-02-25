# Streaming Stability and Doc Structure Design

Goal: Fix streaming UI resets during delta updates and preserve document structure in final answer rendering while keeping short summaries softer and less list-heavy.

Context and constraints:
- Streaming deltas update timestamps, so keys derived from timestamp remount components and restart streaming.
- Final answer rendering currently softens headings and lists for all content, which flattens document previews.
- Keep behavior changes minimal and reversible. Avoid API changes.

Approach:
1) Stabilize keys for streaming events
- For workflow.node.output.delta events, use a key derived from stable identifiers: event type, session_id, task_id, parent_task_id, agent_level, iteration. These fields stay stable across chunks.
- For workflow.result.final events, use a stable key based on session_id, task_id, parent_task_id to prevent remounts during final stream updates.
- Preserve existing ordering based on timestamps; only key selection changes.

2) Preserve document structure while softening summaries
- Add a lightweight heuristic to detect document-like content based on markdown length and the count of headings and list markers.
- If document-like, render with default markdown components so headings/lists appear normally.
- If not document-like, apply the summary softening components (div-based headings/lists, stronger bold) to keep final summaries readable without heavy structure.

Testing strategy:
- Add a failing test that ensures delta DOM nodes remain stable across updates.
- Add a failing test that ensures document-like content still renders headings and lists.
- Run targeted vitest for these components, then full lint/tests.

Risks and mitigations:
- Heuristic misclassification: choose conservative thresholds that only bypass softening when structure is clearly document-like.
- Key collisions: include session/task identifiers and iteration to avoid collisions when multiple streams exist.

