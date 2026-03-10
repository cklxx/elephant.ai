# Security Audit

Date: 2026-03-10
Repository: `elephant.ai`
Scope: hardcoded secret scan plus targeted review for SQL injection, command injection, and path traversal in Go/config surfaces.

## Executive Summary

Two real code issues stand out:

1. `read_file`, `write_file`, and `replace_in_file` allow paths outside the workspace/root and can read or overwrite arbitrary host files.
2. Verification commands are executed with `bash -lc` from externally supplied task config, which creates a config-driven command injection surface.

I did **not** find a confirmed SQL injection issue in the reviewed database code.

I also did **not** find committed live API keys in tracked Go/config files after filtering out test fixtures and `${ENV_VAR}` placeholders. However, the local workspace contains a non-tracked `.env` file with real-looking credentials and secrets.

## Findings

### 1. High: Built-in file tools permit path traversal / arbitrary file access outside the workspace

Affected code:

- `internal/infra/tools/builtin/aliases/read_file.go`
- `internal/infra/tools/builtin/aliases/write_file.go`
- `internal/infra/tools/builtin/aliases/replace_in_file.go`
- `internal/infra/tools/builtin/pathutil/path_guard.go`
- `internal/infra/tools/builtin/pathutil/path_guard_test.go`
- `internal/infra/integration/path_injection_e2e_test.go`
- `internal/app/toolregistry/registry_builtins.go`

Why this is a problem:

- The file tools call `pathutil.ResolveLocalPath(...)`.
- `ResolveLocalPath` only normalizes to an absolute path; it does **not** enforce containment within the workspace/root.
- The tests explicitly assert that `../escape.txt` and absolute paths outside the base should resolve successfully.
- The integration test `TestPathInjectionE2E_ReadsOutsideWorkspace` proves an agent can read a secret file outside the workspace.

Impact:

- Arbitrary file read via `read_file`.
- Arbitrary file overwrite/append via `write_file`.
- Arbitrary in-place modification via `replace_in_file`.
- In practice this breaks the expected repository boundary and exposes host files such as SSH keys, cloud credentials, shell history, and service configs.

Evidence:

- `internal/infra/tools/builtin/pathutil/path_guard.go`: `resolveAbsolutePath` returns `filepath.Abs(filepath.Clean(candidate))` without containment enforcement.
- `internal/infra/tools/builtin/pathutil/path_guard_test.go`: traversal and outside-base paths are expected to resolve.
- `internal/infra/integration/path_injection_e2e_test.go`: end-to-end test demonstrates successful read of a secret outside the workspace.

Recommendation:

- Enforce `PathWithinBase` in `ResolveLocalPath` for file tools.
- Treat explicit exceptions as opt-in and narrowly scoped, not default behavior.
- Add negative tests for read/write/replace attempts against `..`, symlink escapes, and absolute paths outside the workspace.

### 2. Medium: Verification commands are shell-interpreted from task config

Affected code:

- `internal/infra/coding/verify.go`
- `internal/infra/coding/managed_executor.go`
- `internal/infra/devops/shadow/agent.go`
- `internal/shared/config/types.go`

Why this is a problem:

- `verify_build_cmd`, `verify_test_cmd`, and `verify_lint_cmd` are read from a `map[string]string` config.
- Those strings are executed with `exec.CommandContext(ctx, "bash", "-lc", command)`.
- This is not argument-safe execution; shell metacharacters, command chaining, substitution, and redirection are all active.
- The config shape is reusable in external/team task configuration (`TeamRoleConfig.Config`) and in shadow-task execution paths.

Impact:

- If an attacker can influence task config, they can execute arbitrary shell commands under the agent process account.
- Even if this is intended for trusted operators, it is a sharp edge with no validation, no allowlist, and no separation between "verification command" and "arbitrary shell".

Evidence:

- `internal/infra/coding/verify.go`: `exec.CommandContext(ctx, "bash", "-lc", command)`.
- `internal/infra/coding/managed_executor.go`: verification plan is derived from task config and executed automatically for coding tasks.
- `internal/infra/devops/shadow/agent.go`: shadow tasks also resolve verification commands from `task.Config`.

Recommendation:

- Prefer structured command execution (`binary + args`) over `bash -lc`.
- If shell is required, restrict this to trusted/admin-only config sources and validate/document that boundary explicitly.
- Consider allowlisting known verification commands (`go test ./...`, `go build ./...`, `./dev.sh lint`) instead of arbitrary shell.

### 3. Low: Insecure development default JWT secret

Affected code:

- `dev.sh`

Why this is a problem:

- The development helper exports `AUTH_JWT_SECRET="${AUTH_JWT_SECRET:-dev-secret-change-me}"`.
- This is acceptable for local development only, but it is a footgun if reused in shared or semi-production environments.

Impact:

- Predictable JWT signing secret if operators start services without overriding the environment.

Recommendation:

- Keep it dev-only, but make the script fail closed outside explicit local-dev modes, or emit a loud warning whenever the fallback secret is used.

## Hardcoded Secrets Scan

### Confirmed committed secrets in tracked Go/config files

None found in the reviewed tracked Go/config files after filtering:

- `${ENV_VAR}` config placeholders such as in `configs/config.yaml`
- GitHub Actions `${{ secrets.* }}`
- test fixtures such as `sk-test-*`, `tok-abc`, and similar dummy values

### Local workspace secrets present in `.env` (not tracked by git)

`git ls-files .env` returned no tracked `.env`, but the local workspace file exists and contains real-looking credentials, including:

- `OPENAI_API_KEY`
- `TAVILY_API_KEY`
- `ARK_API_KEY`
- `CLOUDFLARE_SECRET_ACCESS_KEY`
- `GOOGLE_CLIENT_SECRET`
- `ALEX_BROWSER_BRIDGE_TOKEN`
- `ANYGEN_API_KEY`
- `AUTH_DB_PASSWORD`
- `AUTH_BOOTSTRAP_PASSWORD`

Risk:

- These are not currently committed, but they are present on disk and were copied into the worktree.
- They can still leak through accidental commits, debug output, backups, shell history, or overly broad file-tool access.

Recommendation:

- Rotate any credential that is real and still active.
- Ensure `.env` stays ignored.
- Prefer secret-manager or per-user runtime config for high-value credentials.

## SQL Injection Review

No confirmed SQL injection finding in the reviewed code.

Reviewed hotspot:

- `internal/infra/memory/index_store.go`

Notes:

- The `IN (...)` query in `LookupEmbeddings` is dynamically assembled, but only for placeholder count.
- User values are still supplied as query parameters (`args...`), which is the correct pattern.

## Command Injection Review

Confirmed issue:

- Config-driven verification commands in `internal/infra/coding/verify.go`.

Reviewed but not reported as separate vulnerabilities:

- `internal/infra/tools/builtin/aliases/shell_exec.go` intentionally executes arbitrary shell commands; this is a privileged capability, not an accidental injection sink.
- `internal/infra/external/bridge/executor.go` runs `bash setup.sh`, but the executed script path is derived from configured bridge script locations rather than interpolated shell text.

## Path Traversal Review

Confirmed issue:

- Workspace escape in built-in file tools via `ResolveLocalPath`.

Reviewed safe pattern:

- `internal/infra/tools/builtin/artifacts/attachment_resolver.go` cleans the relative path and rejects paths that escape the attachment store directory with `PathWithinBase`.

## Recommended Next Actions

1. Fix the file-tool workspace escape first; it is already proven by tests and enables direct host file access.
2. Restrict or redesign verification command execution to remove `bash -lc` on configurable strings.
3. Rotate any active credentials in the local `.env` and keep them out of logs/artifacts/worktree copies.
4. Add regression tests that enforce the intended repository boundary for all local file tools.

## Audit Method

- Broad secret-pattern scan over Go/config/script files.
- Targeted review of execution sites using `exec.Command*`.
- Targeted review of SQL call sites using `ExecContext`/`QueryContext`.
- Targeted review of file/path handling and attachment path resolution.
