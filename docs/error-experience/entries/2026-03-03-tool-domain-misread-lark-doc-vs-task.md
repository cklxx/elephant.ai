# Tool Domain Misread: Lark Doc Writing vs Task Management (2026-03-03)

## What happened
User clarified the target tool domain was **Feishu doc writing**, but I was still tracing **task management** paths.

## Impact
Investigation time was spent in the wrong module and delayed root-cause analysis.

## Root cause
I anchored on an earlier phrase ("设置任务") and did not re-bind the active target module immediately after user correction.

## Preventive rule
When user clarifies or corrects tool domain, immediately:
1. Freeze current hypothesis.
2. Re-state the exact target module in one line.
3. Re-run scoped search only in that module before proposing fixes.

## Validation checklist for next time
- Confirm target tool name from code (`*_manage.go`) before edits.
- Ensure first file reads after correction are from the corrected module.

## Metadata
- id: err-2026-03-03-tool-domain-misread-lark-doc-vs-task
- tags: [scope-control, lark, tooling, correction]
- links: []
