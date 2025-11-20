# Final answer summarization redesign

## Context
- The previous "final" tool made the LLM produce a synthetic tool call to end runs. This flow is removed to simplify the termination surface and avoid tool plumbing for final responses.
- Task completion now relies on assistant messages (without tool calls) from the core agent loop. A dedicated summarization step is layered after the core agent completes.

## Goals
- Generate the user-facing final answer by running a lightweight LLM summarization pass over non-system conversation messages.
- Keep summaries short, clear, and easy to scan while preserving key attachments or artifact references.
- Deliver the summary via the existing `final_answer` event channel **as a streaming response** so the UI can render deltas as they arrive.
- Persist the summary with the session record without re-injecting it into the next message loop.

## Proposed summarization function
- **Input selection:**
  - Collect all non-system messages from the finished core agent session (assistant replies, tool results, user turns).
  - Exclude raw system prompts and debug scaffolding to keep the context minimal.
  - Preserve attachment placeholders and attachment metadata so downstream rendering stays aligned with current file/preview handling.
- **Prompt shape:**
  - Instruction block: "You are producing the final user-facing answer. Write a crisp summary with the essential steps, results, and explicit next actions. Prefer bullet points when possible; keep it short."
  - Context block: serialized conversation snippets with role tags; omit system messages.
  - Output guardrails: request under 120–180 words, avoid repetition, and echo attachment placeholders as-is.
- **LLM call:**
  - Use the streaming completion LLM client (no tool schema) with the above prompt so deltas can be surfaced progressively.
  - Fall back to a single-shot completion when streaming is unavailable while preserving the final output shape.
- **Attachments compatibility:**
  - Reuse `decorateFinalResult`/attachment resolution helpers to map placeholders to resolved assets before emitting the terminal summary event.
  - Maintain current attachment/preview payload structure so the front end keeps rendering files and artifacts without change.
- **Session handling:**
  - Store the summarizer output alongside the session/task record for auditing.
  - Do **not** append the summarizer message back into the ReAct loop messages to avoid influencing any follow-up iteration.

## Event transport
- Emit a dedicated `final_answer` event from the summarizer (replacing the former task-loop emission) without requiring streaming deltas from the model.
- Preserve the existing event payload shape (`final_answer`, optional `attachments`, `stop_reason`, duration metadata) so attachment registry and file viewers continue to work.

## TODO
- [x] Introduce a summarizer component that accepts a completed `TaskResult` plus message transcript and emits the compact summary via `final_answer` events.
- [x] Wire the coordinator to run the summarizer after the core agent completes and before persisting the session.
- [x] Add tests covering: exclusion of system messages, attachment placeholder preservation, streaming event emission order, and session storage without polluting the next loop.
- [x] Update front-end mocks (if any) once the new event timing is active.
