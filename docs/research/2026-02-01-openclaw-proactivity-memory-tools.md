# OpenClaw: Proactivity, Long-Term Memory, Tools, macOS Integration

Updated: 2026-02-01

## Scope
These notes summarize how OpenClaw enables proactive behavior and long-term memory, what tools are available, and why the macOS experience is strong. Sources are the official OpenClaw documentation and the OpenClaw repository README.

## Architecture Snapshot (Public Overview)
- OpenClaw describes itself as a personal AI assistant that can work across multiple channels and render UI elements (Canvas) in responses.
- The system uses a Gateway as a control plane and connects agents and tools through it.

## Proactivity: Time + Event Engines

### Cron jobs (time-driven)
- Cron is the built-in Gateway scheduler. Jobs are persisted under `~/.openclaw/cron/` and can be triggered on a schedule.
- Cron can run as a "main" session (with a system event and heartbeat in the active conversation) or as an isolated session (with a `cron:<jobId>` turn) to separate side effects.
- Jobs can request immediate wake-up or wait for the next heartbeat; outputs can be sent back into the chat session.

### Hooks (event-driven)
- Hooks are event-driven scripts: they define commands that run when OpenClaw emits events (e.g., `/new`, `/reset`, `/stop`, `gateway:startup`).
- Hooks are discoverable from the workspace and user config directories; OpenClaw ships bundled hooks, including session memory and command logging.

### Combined model
- Proactivity is achieved by combining timed jobs (Cron) with event-driven triggers (Hooks) in the Gateway layer, so behavior does not depend on the model remembering to act.

## Long-Term Memory Model

### Storage
- Memory is stored as Markdown files in the workspace. The model only "remembers" what has been written to disk.
- By default, OpenClaw uses daily log files under `memory/YYYY-MM-DD.md` and a long-term `MEMORY.md` file. On startup, it loads the current day and the previous day.

### Memory plugins
- A memory plugin controls recall and capture. The default is `memory-core`, and a LanceDB-backed plugin (`memory-lancedb`) can be used for semantic retrieval and automatic recall/capture.

### Auto-flush prompts
- When the conversation approaches a compaction boundary, OpenClaw triggers a silent memory flush to prompt the model to persist stable facts.

### CLI support
- The CLI exposes memory operations, including status, index, and search, to build and query the memory index.

## Tooling

### Typed tools and allow/deny
- OpenClaw uses typed tools and allows enable/deny lists for limiting tool access by profile.

### Tool groups (shorthands)
- The tools page documents group shorthands (runtime, fs, sessions, memory, web, ui, automation, messaging, nodes, openclaw), which map to concrete tools.

### Inventory highlights
- Core tools include `apply_patch`, `exec`, `process`, `web_search`, `web_fetch`, `browser`, `canvas`, `nodes`, `image`, `message`, `cron`, `gateway`, session tools (`sessions_*` / `session_status`), and `agents_list`.
- `exec` supports multiple hosts (sandbox, gateway, node), enabling platform-specific execution through node hosts.

## Why macOS Works Well
- The macOS companion app is a menu bar application that centralizes permissions (TCC), starts and manages local node hosts, and exposes macOS-native tools (Canvas, Camera, Screen Recording, `system.run`).
- The companion app can connect to a local or remote Gateway, with the node host remaining on the Mac for local execution and permissions.
- WebChat is embedded in the macOS app (SwiftUI), providing a native UI that directly accesses system capabilities.

## Practical Takeaways
- Proactivity is reliably externalized to the Gateway (Cron + Hooks), which reduces dependence on model self-reminders.
- Long-term memory is file-first, with optional semantic retrieval via plugins and explicit CLI support for indexing and search.
- macOS usability is driven by a first-party companion app that consolidates permissions, node-host tooling, and UI.

## Sources
- https://github.com/openclaw/openclaw
- https://docs.openclaw.ai/automation/cron-jobs
- https://docs.openclaw.ai/hooks
- https://docs.openclaw.ai/concepts/memory
- https://docs.openclaw.ai/cli/memory
- https://docs.openclaw.ai/tools
- https://docs.openclaw.ai/platforms/macos
- https://docs.openclaw.ai/platforms/mac/webchat
