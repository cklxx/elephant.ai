# Plan: Propagate subagent attachments to parent tool context

## Goal
Ensure attachments generated inside subagent runs are captured and surfaced to the main agent, so `artifacts_list` can see them.

## Context
- Subagent events are wrapped into `WorkflowEventEnvelope` before reaching the subtask listener.
- Attachment collector currently reads only AttachmentCarrier events; envelopes do not expose attachments.

## Steps
1. Expose attachments from `WorkflowEventEnvelope` via `GetAttachments` so it satisfies `AttachmentCarrier`.
2. Add a test ensuring subtask listener captures attachments from envelope payloads.
3. Run full lint + tests.

## Progress Log
- 2026-01-27: Implemented `WorkflowEventEnvelope.GetAttachments` and added subtask listener test coverage.
