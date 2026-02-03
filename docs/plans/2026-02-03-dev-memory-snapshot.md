# Plan: Dev Memory Snapshot Endpoint

Owner: cklxx  
Date: 2026-02-03

## Goal
Restore `/dev/conversation-debug` memory loading by providing a dev API that reads Markdown memory (long-term + daily logs).

## Scope
- Add `GET /api/dev/memory` dev-only endpoint.
- Wire memory engine into API handler/router.
- Provide snapshot payload (user_id, long_term, daily[]).
- Add unit test for handler.

## Non-Goals
- Change memory storage layout or indexing behavior.
- Add new UI features beyond fixing the 404.

## Plan of Work
1) Add API handler + route; wire memory engine.
2) Implement daily log listing + long-term load.
3) Add unit test.

## Test Plan
- `go test ./internal/server/http`
- `./dev.sh test`
- `./dev.sh lint`

## Progress
- [x] API handler + route wiring
- [x] Daily/long-term snapshot implementation
- [x] Unit test added
- [x] Tests executed (dev.sh test failing in lark/testing loop-selfheal-smoke; lint ok)
