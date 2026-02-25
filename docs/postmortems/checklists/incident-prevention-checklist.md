# Incident Prevention Checklist

Use this list before closing a high-impact incident.

## Boundary & Scope
- [ ] The fix has an explicit scope boundary (channel/mode/tenant/user/session).
- [ ] A negative test exists for "must not apply here".
- [ ] A positive test exists for "must apply here".

## Regression Guards
- [ ] At least one automated test reproduces the original failure path.
- [ ] Test names encode the boundary condition (e.g. non-unattended vs unattended).
- [ ] Existing nearby tests were reviewed for missing assertions on the same boundary.

## Prompt/Context Safety
- [ ] Any injected context has a size budget or gating condition.
- [ ] Cross-channel/context leakage risk has been explicitly assessed.
- [ ] Runtime-only context is not injected into user-facing default flows.

## Documentation & Memory
- [ ] Full postmortem created under `docs/postmortems/incidents/`.
- [ ] Error-experience entry + summary entry added.
- [ ] At least one prevention pattern documented in guides/checklist/template.

## Validation
- [ ] Relevant package tests pass.
- [ ] Repo lint/test gates were executed (or explicit blocker recorded).
- [ ] Mandatory code review completed and P0/P1 issues resolved.

