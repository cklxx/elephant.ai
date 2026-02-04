# Plan: Proactive UX — `send_message` + Final Answer Review

Date: 2026-02-04
Branch: `eli/proactive-send-message-final-review`

## Goals
- Add a channel-agnostic `send_message` tool (Lark-backed for now).
- Add an automatic “final answer review” extra iteration (heuristic, max 1 by default).
- Update default policy + docs to encourage proactive messaging and tool exploration.

## Work Items (tracked)
- [x] Add `send_message` tool + tests; register in registry; document in TOOLS catalog.
- [x] Add `runtime.proactive.final_answer_review` config + merge + docs.
- [x] Wire config into ReactEngine and implement runtime heuristic + tests.
- [x] Update `configs/context/policies/default.yaml` soft preferences.
- [x] Run `go test ./...` and repo lint; fix only relevant failures.
- [ ] Merge back to `main` (prefer fast-forward).
