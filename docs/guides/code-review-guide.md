# Code Review Guide

Updated: 2026-03-03

Mandatory code review process before every commit. Covers SOLID architecture, security, code quality, and boundary conditions.

---

## When to Run

After lint + tests pass, **before any commit or merge**. No exceptions.

```bash
python3 skills/code-review/run.py review
```

---

## Blocking Rules

| Severity | Action |
|----------|--------|
| **P0** | Must fix before commit. Security vulnerabilities, data loss, correctness bugs. |
| **P1** | Must fix before commit. Significant quality or architecture issues. |
| **P2** | Create follow-up task. Non-critical improvements. |
| **P3** | Optional. Style preferences, minor suggestions. |

---

## Review Dimensions

### SOLID Architecture

| Principle | Checklist |
|-----------|-----------|
| **SRP** | One file/struct per responsibility. Functions under 50 lines. |
| **OCP** | New behavior via new types/interfaces, not expanding switch chains. |
| **LSP** | Subtypes can genuinely replace base types. |
| **ISP** | Go interfaces prefer 3-5 methods max. Split fat interfaces by consumer use case. |
| **DIP** | Business logic depends on interfaces, not concrete implementations. |

### Code Quality

- **Error handling**: never swallow errors; use `%w` wrapping; include context in error messages.
- **Performance**: no N+1 queries; cache static computations; no unbounded collections; use `strings.Builder` in concat loops.
- **Boundary conditions**: nil/zero-value checks; empty collection guards; integer overflow; Unicode correctness.
- **Observability**: structured logs on key operations; request ID in error logs; latency/throughput metrics on critical paths; trace context propagation.

### Security

- **Input/output safety**: parameterized SQL/queries; path traversal validation (`filepath.Clean` + prefix check); SSRF prevention.
- **Auth/authz**: IDOR checks; permission validation on sensitive operations; JWT algorithm validation.
- **Sensitive data**: no hardcoded secrets; no PII in logs; `.env` in `.gitignore`.
- **Race conditions**: shared maps require `sync.Map` or `sync.RWMutex`; read-modify-write in transactions; TOCTOU awareness.
- **Go-specific**: goroutine exit mechanisms (context cancel, done channel); goroutine leak prevention; `context.Background()` not used where request context should be passed.

---

## Review Workflow

1. Run `alex dev lint` + `alex dev test` — all must pass.
2. Run `python3 skills/code-review/run.py review`.
3. Read the generated report.
4. Fix all P0/P1 findings.
5. Re-run lint + test after fixes.
6. Create follow-up tasks for P2 findings.
7. Commit only when clean.

---

## References

- Skill definition: `skills/code-review/SKILL.md`
- SOLID checklist: `skills/code-review/references/solid-checklist.md`
- Quality checklist: `skills/code-review/references/code-quality-checklist.md`
- Security checklist: `skills/code-review/references/security-checklist.md`
