# User History Recall, Search, and Blending Plan

## Goals
- Surface relevant prior exchanges to ground new tasks with historical context.
- Generate lightweight search seeds from recalled snippets so the RAG gate can bootstrap retrieval even when the current prompt is sparse.
- Avoid regressions in latency or privacy by bounding the amount of history we inspect and summarise.

## Pipeline Overview
1. **Query Assembly**
   - Seed with the raw user task, task analysis goal/action name, approach, and retrieval directives.
   - Collapse the assembled text into a whitespace-normalised query string.
2. **Candidate Selection**
   - Iterate over prior `user` messages (skipping non-user sources).
   - Tokenise each message with minimal filtering and calculate overlap ratios against the query tokens (no recency weight).
   - Accept snippets once at least two tokens overlap or the overlap ratios clear `historyMinOverlapRatio`; literal substring matches are always kept.
3. **Snippet Construction**
   - Accept candidates in reverse chronological order until `historyMaxSnippets` is reached.
   - Pair each with its assistant reply when present to preserve conversational continuity.
   - Condense both sides to `historySnippetRuneLimit` runes to enforce budget limits.
4. **Summary & Injection**
   - Emit a system message summarising the recalled exchanges and tag it with `MessageSourceUserHistory` so downstream renderers can highlight it.
   - If no summary text remains after condensation, skip injection entirely.
5. **Seed Harvesting**
   - Use the first five deduplicated tokens extracted during candidate selection as deterministic search seeds.
   - Skip bespoke fallback heuristics to keep the implementation lean; empty token sets simply omit a seed.
6. **RAG Gate Blending**
   - Append history-derived seeds to both the signals fed into the RAG gate and the final directives returned to execution.
   - Allow the seeds to supply the base query when the task analysis goal is empty.

## Safeguards
- Ignore empty messages and non-user sources to prevent leaking internal state.
- Trim duplicate seeds case-insensitively to avoid noisy queries.
- Cap the number of snippets so the inserted system message stays compact.

## Future Enhancements
- Persist seed-to-result success metrics to determine whether the simplified overlap ratio needs further tuning.
- Re-introduce semantic weighting (recency or embeddings) only if the streamlined approach proves insufficient in practice.
