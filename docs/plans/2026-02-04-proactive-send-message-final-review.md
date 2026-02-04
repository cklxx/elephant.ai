# Plan: Proactive UX — proactive messaging + Final Answer Review

Date: 2026-02-04
Branch: `eli/proactive-send-message-final-review`

## Goals
- Add a proactive messaging tool for Lark (`lark_send_message`).
- Add an automatic “final answer review” extra iteration (heuristic, max 1 by default).
- Update default policy + docs to encourage proactive messaging and tool exploration.

## Work Items (tracked)
- [x] Add/adjust messaging tool support for Lark (`lark_send_message`) and document in TOOLS catalog.
- [x] Add `runtime.proactive.final_answer_review` config + merge + docs.
- [x] Wire config into ReactEngine and implement runtime heuristic + tests.
- [x] Update `configs/context/policies/default.yaml` soft preferences.
- [x] Run `go test ./...` and repo lint; fix only relevant failures.
- [x] Merge back to `main` (prefer fast-forward).

## Notes
- 2026-02-04: Dropped duplicated UI `send_message`; keep `lark_send_message` only.
