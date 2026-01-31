# Plan: Lark emoji reactions for event start/end

## Context
- Need random emoji reactions for Lark events; different event start/end should use different emoji.
- Lark reactions currently only use `react_emoji` and pre-analysis emoji events.

## Steps
1. Inspect Lark gateway event flow + existing emoji reaction logic; define event→emoji pools. ✅
2. Add tests for event emoji selection and reaction interceptor behavior (TDD). ✅
3. Implement emoji picker + event reaction interceptor; wire into Lark gateway. ✅
4. Update config/docs if behavior changes; ensure YAML examples only. ✅
5. Run full lint + tests; update plan status; commit in small increments. ✅
