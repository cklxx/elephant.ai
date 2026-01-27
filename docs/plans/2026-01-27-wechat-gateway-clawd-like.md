# Plan: WeChat gateway integration (2026-01-27)

## Goal
- Add a WeChat gateway (QR login → message → agent → reply) modeled after clawd.bot’s gateway + channel design.
- Support a “computer mode” that uses the user’s machine via bash + filesystem tools.

## Pre-checks
- Reviewed `docs/guides/engineering-practices.md`.

## Plan
1. Research clawd.bot gateway/channel model and openwechat APIs (QR login, message handling, user IDs).
2. Extend configuration schema for `channels.wechat` and load defaults in server bootstrap.
3. Implement the WeChat gateway package + server startup hook, including session mapping and tool-mode selection.
4. Document usage/config in reference docs (YAML-only examples).
5. Update tests, run full lint + test, and commit.

## Progress
- 2026-01-27: Plan created; engineering practices reviewed.
- 2026-01-27: Research completed (clawd.bot docs, openwechat docs).
- 2026-01-27: Implemented WeChat gateway, channel config, and unified ~/.alex storage defaults.
- 2026-01-27: Ran `./dev.sh lint` and `./dev.sh test` (both pass; go test emits ld warnings on macOS).
