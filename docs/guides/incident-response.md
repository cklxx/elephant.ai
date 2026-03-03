# Incident Response

Updated: 2026-03-03

Process for handling high-impact regressions, cross-channel leakage, and prompt/context safety incidents.

---

## Trigger Conditions

Create an incident file when any of these occur:
- **High-impact regression**: user-facing functionality broken by a code change.
- **Cross-channel leakage**: data or context from one channel (Lark, WeChat, CLI, web) appearing in another.
- **Prompt/context safety incident**: unintended context injection, manipulative framing, or approval gate bypass.

---

## Workflow

### 1. Open Incident (within 1 working day)

Create: `docs/postmortems/incidents/YYYY-MM-DD-short-incident-slug.md`

Use the template at `docs/postmortems/templates/incident-postmortem-template.md`.

### 2. Fill All Mandatory Sections

| Section | Requirements |
|---------|-------------|
| **What happened** | Symptom, trigger condition, detection channel |
| **Impact** | User-facing impact, internal impact, blast radius |
| **Timeline** | Exact dates, not relative timestamps |
| **Root cause** | Technical root cause + process root cause + why existing checks missed it |
| **Fix** | Code/config changes, scope/rollout strategy, verification evidence |
| **Prevention actions** | Each with owner, due date, and validation method |
| **Follow-ups** | Open risks, deferred items |
| **Metadata** | id, tags, links to error-experience entries |

### 3. Create Experience Records

- Add an **error-experience entry** in `docs/error-experience/entries/`.
- Add an **error-experience summary** in `docs/error-experience/summary/entries/`.
- Include at least one prevention action that is **testable** (test/lint/check/process gate).
- Link the implementation PR/commit and validation evidence.

### 4. Complete Prevention Checklist

Before closing the incident, complete every item in `docs/postmortems/checklists/incident-prevention-checklist.md`:

**Boundary & Scope**
- The fix has an explicit scope boundary (channel/mode/tenant/user/session).
- A negative test exists for "must not apply here".
- A positive test exists for "must apply here".

**Regression Guards**
- At least one automated test reproduces the original failure path.
- Test names encode the boundary condition.
- Existing nearby tests reviewed for missing assertions on the same boundary.

**Prompt/Context Safety**
- Any injected context has a size budget or gating condition.
- Cross-channel leakage risk explicitly assessed.
- Runtime-only context not in user-facing default flows.

**Documentation & Memory**
- Full postmortem created.
- Error-experience entry + summary added.
- Prevention pattern documented in guides/checklist/template.

**Validation**
- Package tests pass.
- Lint/test gates executed.
- Code review completed with P0/P1 resolved.

---

## File Locations

| Artifact | Path |
|----------|------|
| Incident reports | `docs/postmortems/incidents/` |
| Postmortem template | `docs/postmortems/templates/incident-postmortem-template.md` |
| Prevention checklist | `docs/postmortems/checklists/incident-prevention-checklist.md` |
| Error experience entries | `docs/error-experience/entries/` |
| Error experience summaries | `docs/error-experience/summary/entries/` |
