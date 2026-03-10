# Code Review Guide

Updated: 2026-03-10

Run code review after lint and tests, and before every commit or merge.

```bash
python3 skills/code-review/run.py review
```

## Severity

| Severity | Action |
|----------|--------|
| `P0` | Fix before commit. Security, data loss, or clear correctness failure. |
| `P1` | Fix before commit. Significant architecture, reliability, or maintainability issue. |
| `P2` | Do not block the commit, but create a follow-up task. |
| `P3` | Optional. |

## Review Checklist

### Architecture

- Responsibilities are not mixed across files, types, or layers.
- `internal/**` keeps delivery -> application -> domain -> infra boundaries clean.
- Business logic depends on interfaces or ports, not concrete adapters.
- New behavior extends existing design cleanly instead of growing switch chains and special cases.

### Correctness

- Errors are handled and wrapped with useful context.
- Edge cases are covered: zero values, empty inputs, nil, overflow, Unicode, and partial failure.
- Concurrency is safe: no races, leaks, or missing cancellation.
- Tests prove the changed behavior.

### Security And Data Safety

- Queries are parameterized.
- File and network inputs are validated for traversal and SSRF risks.
- Auth and permission checks exist where data or actions are sensitive.
- Secrets and PII are not hardcoded or logged.

### Performance And Operations

- No obvious N+1 work, unbounded memory growth, or accidental hot-path allocations.
- Critical paths keep logs, metrics, and trace context useful.
- Background work has a clear lifecycle and shutdown path.

## Workflow

1. Run relevant lint and tests.
2. Run `python3 skills/code-review/run.py review`.
3. Fix every `P0` and `P1`.
4. Re-run validation after fixes.
5. Create follow-up work for every `P2`.
6. Commit only when the remaining findings are acceptable.

## References

- `skills/code-review/SKILL.md`
- `skills/code-review/references/solid-checklist.md`
- `skills/code-review/references/code-quality-checklist.md`
- `skills/code-review/references/security-checklist.md`
