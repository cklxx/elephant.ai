# State Management Guide

## Decision Matrix

| Use Case | Preferred Tool | Notes |
| --- | --- | --- |
| Cross-component client state | Zustand | Use for session state, event streams, and shared UI state. Keep stores small and focused. |
| Server state / async queries | React Query | Use for task execution, session lists, and cacheable API data. |
| Local UI state | useState / useReducer | Use for dialog open state, form inputs, and component-local toggles. |

## Rules of Thumb

- Prefer local state first; lift only when truly shared.
- Keep Zustand stores event-driven and normalized; avoid large derived blobs inside the store.
- Keep React Query keys stable and colocated with the consuming component/hook.
- Document any cross-cutting state in component or hook README when it grows beyond ~100 LOC.
