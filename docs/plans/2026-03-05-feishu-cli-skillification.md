# 2026-03-05 Feishu CLI Skillification

## Goal
- Convert current Feishu/Lark skill tool paths to a unified local CLI interface.
- Borrow command surface ideas from `riba2534/feishu-cli` while implementing functionality in-repo.
- Prioritize robust authorization (tenant token + OAuth code flow + refresh + precheck).
- Expose CLI through skills so agent can execute local Feishu operations directly.

## Constraints
- Keep existing skill action names stable to avoid upstream agent routing regressions.
- Authorization guidance must return official Feishu OAuth URL (no local relay URL surfaced as primary guidance).
- Provide progressive help so LLM can discover usage incrementally.

## Plan
1. Add `scripts/cli/feishu/feishu_cli.py` with:
   - `help` (top/module/action)
   - `auth` (status/tenant_token/oauth_url/exchange_code/refresh_user)
   - `tool` (module + action dispatch)
   - `api` (raw Open API fallback)
2. Add robust auth storage + cache utilities inside CLI implementation.
3. Refactor `scripts/skill_runner/lark_auth.py` to delegate auth + API calls to new CLI internals.
4. Refactor Feishu-related skills to use shared CLI-backed API layer (remove duplicated HTTP/token logic).
5. Add an aggregate skill `skills/feishu-cli/` for agent local use of help/auth/tool/api.
6. Add/adjust tests for CLI help/auth and skill integrations.
7. Run lint + targeted/full tests, run mandatory code review skill, then commit.

## Progress
- [x] Task analysis and target module inventory
- [x] Worktree created
- [x] CLI implementation
- [x] Skill integration
- [x] Tests
- [x] Lint/test/review gates
- [ ] Commit + merge

## Validation
- Targeted/changed Python tests passed:
  - `pytest -q scripts/cli/feishu/tests/test_feishu_cli.py scripts/skill_runner/tests/test_feishu_cli.py skills/calendar-management/tests/test_calendar.py skills/contact-lookup/tests/test_contact_lookup.py skills/doc-management/tests/test_doc_management.py skills/wiki-knowledge/tests/test_wiki_knowledge.py skills/drive-file/tests/test_drive_file.py skills/bitable-data/tests/test_bitable_data.py skills/meeting-automation/tests/test_meeting_automation.py skills/email-lark/tests/test_email_lark.py skills/sheets-report/tests/test_sheets_report.py skills/okr-native/tests/test_okr_native.py skills/feishu-cli/tests/test_feishu_cli_skill.py` (35 passed)
- `alex dev lint` failed in this environment because web lint dependency is missing:
  - `sh: eslint: command not found`
- `alex dev test` surfaced unrelated pre-existing failures in Lark channel suites before completion:
  - `internal/delivery/channels/lark TestBuildModelListSkipsProvidersWithoutCredentials`
  - `internal/delivery/channels/lark/testing TestScenarios/teams_timeout`
- Mandatory review executed:
  - `python3 skills/code-review/run.py '{"action":"review"}'`
