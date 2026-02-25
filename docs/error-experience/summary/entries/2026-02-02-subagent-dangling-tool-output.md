Summary: Subagent snapshots pruned the subagent tool call but left its tool output, yielding Responses inputs with a function_call_output lacking a matching call and triggering HTTP 400 rejections.
Remediation: Remove tool-output messages tied to the pruned subagent call ID and add regression tests.
