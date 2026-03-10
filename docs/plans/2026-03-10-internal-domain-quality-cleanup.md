Status: completed

# Internal Domain Quality Cleanup

## Goal

Audit `internal/domain/` for dead code, unused interfaces, redundant types, overlong functions, and repeated logic. Remove or simplify only changes that are clearly safe and locally verifiable.

## Plan

1. Scan `internal/domain/` for exported declarations without callers, stale TODO/FIXME comments, and repeated logic patterns.
2. Apply focused cleanups with matching test updates where behavior is preserved.
3. Run review and relevant tests, then commit, merge, and push.

## Outcome

- Removed unused exported interfaces `task.TaskSchemaStore`, `task.TaskLifecycleStore`, `task.TaskLeaseStore`, `task.TaskQueryStore`, and `task.TaskAuditStore` by inlining the only surviving `task.Store` contract.
- Deleted dead exported interfaces `materialregistry/ports.EventPublisher` and `agent/ports/agent.AttachmentCarrier`, both of which had no callers in the repository.
- Simplified `AttachmentStoreMigrator.Normalize` by collapsing duplicate hosted-attachment branches.
- Removed duplicate JSON marshalling in workflow snapshot truncation by reusing serialized output bytes.
