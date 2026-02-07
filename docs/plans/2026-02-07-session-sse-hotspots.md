# Plan: Inspect session list + SSE streaming hotspots

## Goal
Identify unoptimized runtime hotspots in session list APIs and web SSE streaming/render paths, then report concrete locations and safe optimizations with tests.

## Steps
1. Locate session list API implementations and data access patterns.
2. Trace SSE streaming/render paths in the web frontend and event pipeline.
3. Summarize hotspots, propose safe optimizations, and outline tests to add.

## Progress
- [x] Step 1: Locate session list API implementations and data access patterns.
- [x] Step 2: Trace SSE streaming/render paths in the web frontend and event pipeline.
- [x] Step 3: Summarize hotspots, propose safe optimizations, and outline tests to add.
