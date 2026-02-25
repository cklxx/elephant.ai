Summary: Kernel-only alignment context leaked into normal Lark prompt assembly, causing oversized system messages and context pollution.
Remediation: Gate injection by unattended runtime context, add dual-path boundary tests, and standardize postmortem checklist/mechanism.

## Metadata
- id: errsum-2026-02-25-kernel-context-leak
- tags: [summary, kernel, lark, context-boundary]
- derived_from:
  - docs/error-experience/entries/2026-02-25-kernel-alignment-context-leak-into-lark-session.md
