# Tools-to-Skills Optimization Plan

> **Created:** 2026-02-09
> **Status:** Phase 0 In Progress
> **Branch:** `feat/tools-to-skills`

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

## Phase 2: Update Skill Engine (PENDING)

- [ ] SkillMatcher reads new SKILL.md format (requires_tools, max_tokens: 200)
- [ ] Prompt injection: skill name+description only during scan; full SKILL.md on match
- [ ] `bash: python3 skills/xxx/run.py` pattern in execution

## Phase 3: Regression + Cleanup (PENDING)

- [ ] Foundation eval suite regression
- [ ] Lark scenario tests pass
- [ ] Delete deprecated Go tool packages
- [ ] Update documentation
