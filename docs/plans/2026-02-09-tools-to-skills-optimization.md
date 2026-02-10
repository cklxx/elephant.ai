# Tools-to-Skills Optimization Plan

> **Created:** 2026-02-09
> **Status:** Phase 4 Complete
> **Branch:** `tools-to-skills-final`

## Goal

Reduce ~75 Go builtin tools to **6 core tools** + **30 declarative Python skills**, following PI Agent's architecture (4 tools → everything else via skills).

## Architecture (v3)

### 6 Core Tools (Keep as Go)

| Tool | Description | Replaces |
|------|-------------|----------|
| `read` | File read | file_read |
| `write` | File write | file_write |
| `edit` | File edit | file_edit |
| `bash` | Shell execution + **skill invocation** | shell_exec + all tool calls via `python3 skills/xxx/run.py` |
| `channel` | Lark/WeChat SDK operations | lark_*, wechat_* (WebSocket, can't go through bash) |
| `browser` | Chrome DevTools Protocol | browser_action (CDP, can't go through bash) |

### Skill Format (Dual-File)

```
skills/<name>/
├── SKILL.md    # YAML frontmatter (triggers, description) + brief usage
├── run.py      # Python execution logic, called via bash
└── tests/
    ├── __init__.py
    └── test_<name>.py
```

- **SKILL.md**: ~200 max_tokens for discovery. LLM sees `name + description` (~10 tok/skill).
- **run.py**: `python3 skills/<name>/run.py '{"action":"...", ...}'` → JSON stdout.
- LLM-assisted skills: run.py collects inputs + provides `*_prompt` field for LLM guidance.

### Token Savings

| Before | After | Savings |
|--------|-------|---------|
| ~75 tools × 200 tok = 15,000 tok | 6 tools × 200 tok = 1,200 tok | **92%** |
| Skills inject full workflow text | Skills inject name+description only | |

## Phase 0: Python Skill Framework (COMPLETED)

### Commits

1. `d8ac5755` — Skill framework + CLI wrappers + Phase 0 first batch (5 skills)
2. `4f9b8070` — Batch 1+2: 12 skills (run.py + tests)
3. `0c6afc64` — Batch 3: 3 hybrid skills (run.py + tests)
4. `ffbf791d` — Batch 4: 10 new domain skills (SKILL.md + run.py + tests)

### All 30 Skills

| # | Skill | Category | Go Tools Replaced |
|---|-------|----------|-------------------|
| 1 | deep-research | Web search | tavily → bash |
| 2 | calendar-management | Lark API | calendar_create/query/update/delete |
| 3 | image-creation | ARK API | image_generate, image_to_image |
| 4 | social-trends | Web scraping | douyin_hot |
| 5 | timer-management | File-based | set_timer, list_timers, cancel_timer |
| 6 | okr-management | File-based | okr_read, okr_write |
| 7 | moltbook-posting | HTTP API | moltbook_* |
| 8 | diagram-to-image | CLI (mmdc) | diagram_render |
| 9 | video-production | ARK API | seedance_* |
| 10 | auto-skill-creation | Scaffolding | (new) |
| 11 | json-render-templates | Pure data | json_render_* |
| 12 | a2ui-templates | Pure data | a2ui_* |
| 13 | code-review | Git + LLM | (skill upgrade) |
| 14 | meeting-notes | Input + LLM | (skill upgrade) |
| 15 | email-drafting | Input + LLM | (skill upgrade) |
| 16 | best-practice-search | Web + local | (skill upgrade) |
| 17 | research-briefing | Web search | (skill upgrade) |
| 18 | ppt-deck | Template + LLM | (skill upgrade) |
| 19 | self-test | Go test runner | (skill upgrade) |
| 20 | eval-systematic-optimization | Eval runner | (skill upgrade) |
| 21 | scheduled-tasks | File-based | scheduler_create/delete/list_jobs |
| 22 | artifact-management | File-based | artifacts_write/list/delete |
| 23 | memory-search | File-based | memory_search, memory_get |
| 24 | task-delegation | File-based | acp_executor |
| 25 | background-tasks | Subprocess | bg_collect, bg_dispatch |
| 26 | desktop-automation | osascript | applescript_run_* |
| 27 | web-page-editing | Pure Python | html_edit |
| 28 | config-management | File-based | config_manage |
| 29 | music-discovery | iTunes API | music_play |
| 30 | lark-messaging | Lark API | send_message |

### Test Coverage

- **307 tests**, all passing
- Every run.py has unit tests with mocked external deps

## Phase 1: SkillMode Config + Core Tool Registry (IN PROGRESS)

### Step 1: SkillMode Config Flag + Core Tools (COMPLETED)

**Commit:** (pending)

Added `SkillMode bool` config flag through all layers:
- `RuntimeConfig`, `Overrides`, `RuntimeFileConfig` (types.go, file_config.go)
- Env var: `ALEX_SKILL_MODE` (runtime_env_loader.go)
- Override + file loader handlers (overrides.go, runtime_file_loader.go)
- `toolregistry.Config`, `di.Config`, `di.ConfigFromRuntimeConfig` wiring

Created `registerSkillModeCoreTools()` + `registerSkillModePlatformTools()`:
- **21 tools** registered (down from **55**, 62% reduction)
- Core tools: read_file, write_file, replace_in_file, shell_exec, execute_code, browser_action
- UI: plan, clarify, request_user
- Memory: memory_search, memory_get
- Web: web_search
- Session: skills
- Lark: all 8 tools (channel consolidation deferred to Step 2)
- Removed: grep, ripgrep, find, todo_*, apps, music_play, artifacts_*, a2ui_emit,
  pptx_from_images, acp_executor, config_manage, html_edit, web_fetch, douyin_hot,
  text_to_image, image_to_image, video_generate, diagram_render, okr_*, timers,
  schedulers, browser_info/screenshot/dom, list_dir, search_file, write_attachment

Tests: 2 new tests (skill mode + skill mode with lark-local), all passing.

### Step 2: Consolidate Lark tools → channel (PENDING)

- [ ] Create `channel_tool.go` — single tool with action dispatch for all Lark SDK ops
- [ ] Replace 8 individual lark tools with 1 `channel` tool

### Step 3: Remove deprecated Go tool packages (PENDING)

- [ ] Delete unused Go tool packages once skill coverage is verified

## Phase 2: Update Skill Engine (COMPLETED)

**Commit:** `a5531ed4` — feat(skills): add Python skill detection + requires_tools parsing

- [x] `RequiresTools []string` field parsed from YAML frontmatter
- [x] `HasRunScript bool` detected via `run.py` alongside SKILL.md
- [x] `[py]` marker in IndexMarkdown for Python skills
- [x] `<type>python</type>` + `<exec>` in AvailableSkillsXML for Python skills
- [x] Skills tool metadata enriched with `type`, `exec`, `requires_tools`
- [x] 4 new tests covering all detection paths (22 total skills tests pass)

## Phase 3: Regression + Cleanup (COMPLETED)

- [x] Full `go build ./...` — compiles (only pre-existing sqlite-vec cgo warnings)
- [x] Full `go test ./...` — all packages pass
- [x] Full `go vet ./...` — clean (same sqlite-vec warnings only)
- [x] Python skill tests — 275 tests pass (run per-skill directory)
- [x] Delete deprecated Go tool packages (completed in Phase 4)
- [x] Update documentation (completed in Phase 4)

## Phase 4: Final Migration (COMPLETED)

**Branch:** `tools-to-skills-final`

### Batch 1 — Consolidate 8 Lark tools → `channel` (commit `96fc9793`)
- Created unified `channel` tool with `action` parameter dispatch
- Actions: `send_message`, `upload_file`, `history`, `create_event`, `query_events`, `update_event`, `delete_event`, `list_tasks`, `create_task`, `update_task`, `delete_task`
- Deleted 8 individual lark tool files + tests

### Batch 2 — Remove SkillMode flag (commit `acffb9a2`)
- Deleted `SkillMode` config flag from all layers (RuntimeConfig, Overrides, env loader, file loader, DI)
- Renamed `registerSkillModeCoreTools()` → `registerBuiltins()`
- Deleted old `registerBuiltins()` full-toolset path + all deprecated register functions
- Cleaned up `degradation_defaults.go` (empty FallbackMap)
- Deleted ~30 unused config fields from DI container

### Batch 3 — Delete ~40 deprecated tool packages (commit `e6d0b8e5`)
- Deleted 11 entire packages: search, config, diagram, execution, media, timer, scheduler, applescript, chromebridge, peekaboo, fileops
- Partial cleanup: web (html_edit, web_fetch, douyin_hot), session (todo_*, apps), aliases (list_dir, search_file, write_attachment), sandbox (attachment, browser_dom, file_list, file_search), browser (info, screenshot, dom), artifacts (tool files only — kept attachment_resolver, attachment_uploader)
- Kept OKR store/types/config (domain types used by scheduler/hooks), deleted only tool files
- Moved `LocalExecEnabled` build-tag flag from execution → aliases package
- ~113 files deleted, ~24,000 lines removed

### Batch 4 — Clean up delivery/output + prompts (commit `ee1bcabe`)
- Slimmed `toolDisplayNames` map to 8 canonical entries
- Rewrote `CategorizeToolName()` and `toolKindForName()` for canonical tools only
- Deleted dead formatters: formatFileOutput, formatSearchOutput, formatWebFetchOutput, formatListFiles, formatTodoList, renderSandboxFileList, renderSandboxFileSearch
- Deleted dead parsers: listFilesSummary, searchSummary, webFetchSummary, sandboxFileListSummary, sandboxFileSearchSummary
- Updated system prompts: Tool Routing Guardrails, Researcher, Security Analyst, Designer, Architect presets
- Updated all tests for canonical tool names
- 8 files changed, -692 lines

### Final Metrics

| Metric | Before | After | Change |
|--------|--------|-------|--------|
| **Go builtin tools** | 58 | 14 | **-76%** |
| **Tool definition tokens** | ~13k | ~2.8k | **-78%** |
| **Python skills** | 30 | 30 | — |
| **Python skill tests** | 307 | 307 | — |
| **Go test suite** | all pass | all pass | — |
| **Files deleted** | — | ~130+ | — |
| **Lines removed** | — | ~25,000+ | — |
