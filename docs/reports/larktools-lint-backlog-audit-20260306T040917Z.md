# Larktools lint backlog audit — 2026-03-06T04:09:17Z

## Scope
- Repo: `/Users/bytedance/code/elephant.ai`
- Focus: `internal/infra/tools/builtin/larktools` docx/task/subtask related paths
- Goal: verify whether existing lint backlog still masks real defects in the current docx/task/subtask change area

## Commands executed
```bash
git status --short --branch
find internal/infra/tools/builtin/larktools -maxdepth 1 -name '*.go' | sort
rg -n --glob '*.go' '(docx|subtask|create_subtask|task)' internal/infra/tools/builtin/larktools
golangci-lint run ./internal/infra/tools/builtin/larktools/... --out-format tab
mkdir -p /tmp/elephant-audit && golangci-lint run ./internal/infra/tools/builtin/larktools/... --out-format line-number > /tmp/elephant-audit/larktools-golangci-lint.txt 2>&1; printf 'exit_code=%s\n' $?; wc -l /tmp/elephant-audit/larktools-golangci-lint.txt
git diff -- internal/infra/tools/builtin/larktools/docx_manage_test.go
rg -n 'func \(t \*larkTaskManage\) (listSubtasks|createSubtask)|func \(t \*larkDocxManage\) writeMarkdown' internal/infra/tools/builtin/larktools
```

## Evidence
- `git status --short --branch`:
  - `## main...origin/main [ahead 1]`
  - modified: `STATE.md`
  - modified: `internal/infra/tools/builtin/larktools/docx_manage_test.go`
  - untracked: `docs/plans/2026-03-06-agent-team-feishu-cli-terminal-integration.md`
  - untracked: `docs/reports/kernel-cycle-2026-03-06T03-08Z.md`
- Scoped lint result:
  - `golangci-lint run ./internal/infra/tools/builtin/larktools/...` exited `0`
  - captured output file `/tmp/elephant-audit/larktools-golangci-lint.txt` has `0` lines
- Relevant entry points confirmed present:
  - `internal/infra/tools/builtin/larktools/channel.go` routes `list_subtasks` / `create_subtask` / `write_doc_markdown`
  - `internal/infra/tools/builtin/larktools/task_manage.go` implements `listSubtasks` and `createSubtask`
  - `internal/infra/tools/builtin/larktools/docx_manage.go` implements `writeMarkdown`

## Findings
1. **No active lint backlog in scoped target.** Within `./internal/infra/tools/builtin/larktools/...`, golangci-lint is currently clean, so there is no evidence that historical lint debt is hiding new docx/task/subtask defects in this package scope.
2. **Current workspace delta is test-only and docx-specific.** The only larktools file changed in `git status` / `git diff` is `docx_manage_test.go`.
3. **Observed change is structurally aligned with real SDK payload shape, not a code smell surfaced by lint.** The test adds `block_id_to_image_urls: []` to the mocked docx convert response and asserts it is present. That reduces parser-shape drift risk for `DocxService.WriteMarkdown`; it does not touch `task_manage.go`, `createSubtask`, or `listSubtasks` runtime logic.
4. **Task/subtask paths are not part of this round's code delta.** They remain relevant audit targets because channel routing points at them, but no new lint signal or local diff points to a fresh regression there.

## Classification of remaining issues
- **Historical lint debt:** none observed in scoped `larktools` package on current HEAD.
- **New lint issues from this round:** none observed.
- **Non-lint residual risk:** moderate test-surface risk only — the docx mock shape was updated, but there was no need in this cycle to re-run the full build-agent style repair/test loop.

## Minimal executable convergence advice
1. Keep the scope narrow: if you want one more deterministic step, run only the docx/task targeted tests, not the full build-agent workflow:
   - `go test -count=1 ./internal/infra/tools/builtin/larktools/... -run 'Docx|TaskManage|Subtask'`
2. If future lint noise returns, preserve the current package-scoped gate as the baseline (`./internal/infra/tools/builtin/larktools/...`) before widening to repo-level lint; otherwise repo-wide debt can blur ownership.
3. After this audit, the next highest-value cleanup is repository hygiene, not lint repair: commit or discard the local `docx_manage_test.go` delta and prune ad-hoc reports if they accumulate.

## Bottom line
Scoped evidence says **no**, the current `larktools` lint backlog is **not** masking real defects for the docx/task/subtask area in this round. The visible change is a docx test-fixture refinement, and scoped golangci-lint is fully green.

