# Lark script auto-rebuild for ma/ta

Date: 2026-02-03
Status: Done
Owner: cklxx

## Goal
- Ensure `./lark.sh ma` / `./lark.sh ta` rebuild and restart when code changes, even if the server is already healthy.

## Plan
1. Review current lark scripts and define a rebuild-staleness signal for main vs test worktrees.
2. Implement build fingerprint tracking + stale-triggered restart for `scripts/lark/main.sh` and `scripts/lark/test.sh`.
3. Add script-level tests for the build fingerprint helper and run full lint/tests.

## Progress
- 2026-02-03: Plan created; reviewing scripts and deciding on staleness detection.
- 2026-02-03: Implemented build fingerprint tracking for main/test and added a regression script for fingerprint staleness.
- 2026-02-03: Ran lark build fingerprint script and full Go test suite; web lint failed (eslint missing).
