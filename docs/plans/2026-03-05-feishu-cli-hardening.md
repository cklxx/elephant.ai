# Plan: Feishu CLI Hardening + Full Coverage Validation (2026-03-05)

## Goals
- Make Feishu CLI parameter semantics clear and independent for LLM/skill use.
- Ensure progressive help fully covers command/module/action discoverability.
- Validate via unit tests + real API calls + prompt injection checks.

## Steps
1. Fix spec consistency bug and alias mapping (completed)
2. Unify request normalization and response context fields for help/auth/tool/api (completed)
3. Strengthen tests for all command branches and alias paths (completed)
4. Add prompt-injection verification for CLI discoverability guidance (completed)
5. Run full verification (pytest + go test + real requests) and summarize matrix (completed)

## Verification commands
- `python3 -m pytest -q scripts/cli/feishu/tests/test_feishu_cli.py skills/feishu-cli/tests/test_feishu_cli_skill.py`
- `python3 -m pytest -q skills/calendar-management/tests skills/contact-lookup/tests skills/doc-management/tests skills/drive-file/tests skills/email-lark/tests skills/meeting-automation/tests skills/okr-native/tests skills/sheets-report/tests skills/wiki-knowledge/tests skills/bitable-data/tests`
- `go test ./internal/app/context ./internal/delivery/channels/... ./internal/app/agent/preparation`
- real CLI calls on help/auth/api/tool representative paths (including auth fallback failure cases)

## Real request matrix (2026-03-05)
- success: `auth.status`, `auth.tenant_token`, `api.contact.scopes`, `calendar.list_calendars`, `contact.list_scopes`, `wiki.list_spaces`, `drive.list_files`, `mail.list_mailgroups`, `meeting.list_rooms`, `okr.list_periods`, `doc.create`, `sheets.create`, `bitable.list_tables(auto_create_app)`
- expected failure with clear reason: `task.list` (missing/invalid user token, Feishu 99991663), `message.history` (`chat_id is required`)
- help regression fixed and verified: `help module --module sheets`, `help action --module okr --action batch_get_okrs`
