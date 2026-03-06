# 2026-03-06 Fix Review Followups Round 2

## Scope

- Fix `reminder-scheduler` no-argv regression and add error-path coverage.
- Fix misleading `alex team run --template list` session output behavior.
- Tighten `team inject` / `team terminal` selector validation for `--role-id` + `--task-id`.
- Sync `reminder-scheduler` SKILL contract with exported actions and parameters.

## Plan

1. Patch `skills/reminder-scheduler` runner and tests around empty/malformed stdin. ✅
2. Patch `cmd/alex/team_cmd.go` validation/output behavior and add CLI tests. ✅
3. Update `skills/reminder-scheduler/SKILL.md` to match the actual CLI contract. ✅
4. Run gofmt, targeted tests, lint/tests, mandatory code review, then commit. ✅

## Verification

- `gofmt -w cmd/alex/team_cmd.go cmd/alex/team_cmd_test.go`
- `python3 -m pytest skills/reminder-scheduler/tests/test_reminder_scheduler.py`
- `go test ./cmd/alex -run 'TestRenderTeamRunCLIOutput_IncludesSessionID|TestTeamRunOutputSessionID_OmitsTemplateListSessions|TestResolveRequestedRoleID|TestSelectTeamRuntimeStatus|TestValidateTeamRunOptions_RequiresExactlyOneInputMode|TestValidateTeamRunOptions_RejectsUnexpectedArgsAndUnsupportedFlagMixes'`
- `go run ./cmd/alex team run --template list`
- `python3 skills/reminder-scheduler/run.py`
- `printf '{' | python3 skills/reminder-scheduler/run.py`
- `alex dev lint`
- `alex dev test`
- `python3 skills/code-review/run.py review`

## Notes

- `alex dev lint` fails on unrelated pre-existing issue: `internal/delivery/server/http/sse_handler_stream.go` has unused `normalizedToolName`.
- `alex dev test` fails on unrelated pre-existing scenario: `alex/internal/delivery/channels/lark/testing` -> `TestScenarios/teams_happy_path`.
