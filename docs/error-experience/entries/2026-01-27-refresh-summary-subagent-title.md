Error: Refreshing the conversation view dropped the core workflow.node.output.summary event and subagent cards lost their preview titles because summary dedupe treated core summaries as duplicates and preview extraction only read top-level subtask_preview.
Remediation: Keep core summaries even when they match final answers, and derive subagent preview from payload/task fallback with coverage.
