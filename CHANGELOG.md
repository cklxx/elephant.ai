# Changelog

All notable changes to ALEX will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Environment-based configuration for web frontend (`.env.development`, `.env.production`)
- research console-style terminal UI layout with persistent input
- User task display in event stream
- Terminal-style event output component with color-coded events
- Research plan approval UI integration
- Agent runtime ports for logger/clock abstraction and a `ReactiveExecutor` contract to enable typed mocking
- Session history pinning and renaming controls with localized copy

### Changed
- **BREAKING**: Completely refactored deployment script (`deploy.sh`)
  - Simplified to focus on local development only
  - Added port conflict detection and cleanup
  - Implemented PID-based process management
  - Added log rotation and health checks
  - Removed Docker and Kubernetes logic
- Refactored web frontend layout following the research console design pattern
  - Three-section flexbox: header (fixed) → output (scrollable) → input (fixed)
  - Persistent task input always visible at bottom
  - Auto-scroll to latest events
  - Horizontal input layout with auto-resize textarea
- Fixed event display to use correct `event_type` field
- Updated all event formatting with proper type narrowing
- Migrated to Zustand v5 API in `useAgentStreamStore`
- React engine now constructed via `ReactEngineConfig`, receiving injected logger/clock dependencies and emitting timestamped events
- Agent coordinator delegates preparation to new execution/task-analysis services, reducing orchestration surface area and stabilising cost tracking

### Fixed
- Input box disappearing after task submission
- Event content not displaying (wrong field access)
- User messages not shown in event stream
- TypeScript compilation errors in event handling
- Infinite re-render loops in SSE hook
- Port conflicts on frontend startup
- Next.js webpack cache corruption

### Removed
- `.env.local` file (replaced with environment-specific files)
- Complex Docker/Kubernetes deployment logic from `deploy.sh`
- Unnecessary summary documentation files

## [0.5.3] - 2025-10-05

### Fixed
- Infinite re-render loop in `useSSE` hook
- TypeScript inference error in `useMemoryStats` hook
- Critical security and code quality issues

## [0.5.2] - Earlier

See git history for earlier changes.

---

## Migration Notes

### Deployment Script Changes
The deployment script has been completely rewritten. New commands:

```bash
./deploy.sh start    # Start backend + frontend
./deploy.sh status   # Check service status
./deploy.sh logs     # Tail logs
./deploy.sh down     # Stop all services
```

Old Docker/Kubernetes commands are no longer supported. For production deployment, use container orchestration directly with the provided Dockerfile.

### Environment Configuration
Frontend now uses environment-specific files:

- **Development**: `web/.env.development` (used by `npm run dev`)
- **Production**: `web/.env.production` (used by `npm run build`)

The `.env.local` file is no longer used. Update your environment configuration in the appropriate file.

### Frontend Layout
The web UI has been redesigned with a terminal-style layout:
- Input is always visible at the bottom
- Events stream above with auto-scroll
- Minimalist design inspired by the research console reference experience
