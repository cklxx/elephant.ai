# 2026-01-27 - refresh drops core summary + subagent preview

- Summary: Refresh replays collapsed core workflow.node.output.summary events and subagent cards lost preview titles when preview only read top-level subtask metadata.
- Remediation: Preserve core summary events and add payload/task preview fallback with tests.
- Resolution: Updated summary dedupe rules and preview extraction; added coverage in ConversationEventStream and EventLine tests.
