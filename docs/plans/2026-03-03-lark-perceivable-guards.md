# 2026-03-03 Lark perceivable guards

## Background
User requires both reliability guards to be enabled and clearly perceivable in chat:
1. Consecutive tool-failure threshold that stops runaway loops.
2. Shutdown/restart interruption notification so in-flight chats are explicitly informed.

## Plan
1. Add a tool-failure guard listener that tracks consecutive `workflow.tool.completed` failures, cancels execution at threshold, and emits a deterministic user-facing message.
2. Add gateway shutdown interruption notifier that cancels running tasks intentionally and sends visible chat notifications.
3. Wire new Lark config field for failure threshold through runtime config parsing and gateway bootstrap.
4. Add/adjust tests for both new behaviors and config loading.
5. Run Lark-targeted tests, then full pre-push validation.

## Progress
- [x] Reviewed runtime + gateway execution/shutdown paths.
- [x] Implemented consecutive tool-failure guard and user-visible termination notice.
- [x] Implemented shutdown/restart interruption notification for running chats.
- [x] Added/updated automated tests.
- [ ] Validation complete (targeted checks passed; full `pre-push` blocked by unrelated existing race-build failure in `internal/delivery/server/app`).
