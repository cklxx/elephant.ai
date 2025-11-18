# User History Recall, Search, and Blending Plan
> Last updated: 2025-11-18


## Goals
- Surface prior exchanges to ground new tasks with historical context while keeping latency predictable.
- Keep recalled turns aligned with what the user and assistant actually said instead of synthesising bespoke snippets.
- Summarise the history only when it would crowd out the active context window.

## Pipeline Overview
1. **Message Filtering**
   - Walk the stored session transcript in order.
   - Drop any `system` role entries, system-prompt sources, or previously injected `user_history` messages.
   - Retain every other message (user, assistant, tool, etc.) as-is so long as it contains textual content, attachments, or tool results.
2. **Token Budget Check**
   - Estimate the token footprint of the filtered turns.
   - If the count exceeds 70% of the configured context window, switch to summarisation; otherwise, return the raw turns verbatim.
3. **LLM Composition**
   - When summarisation is needed, feed the condensed transcript into a low-temperature LLM prompt tailored for memory recall.
   - Emit a single `system` message tagged with `MessageSourceUserHistory` that captures objectives, assistant actions, and pending follow-ups.
4. **Injection**
   - Whether raw or summarised, insert the recalled entries ahead of execution so downstream planners can leverage the context.

## Safeguards
- Removing system prompts before recall prevents redundant persona instructions from polluting the runtime context.
- Summarisation is guarded by a tight timeout and token cap so failures fall back to the raw transcript without blocking execution.
- User-history messages always carry the `MessageSourceUserHistory` source tag for downstream display logic.

## Future Enhancements
- Consider heuristics that prioritise the most recent N turns when the raw transcript is extremely long but still under the threshold.
- Explore richer summary prompts that can call out attachments or tool output metadata explicitly.

