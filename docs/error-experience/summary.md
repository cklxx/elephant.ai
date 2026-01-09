# Error Experience Summary
> Last updated: 2026-01-09

* Go linting can fail if `sum.golang.org` returns 502 or golangci-lint times out; use `GONOSUMDB=...` and increase `--timeout`.
* Git operations can fail due to a stale `.git/index.lock`; remove after ensuring no git process is running.
* Oh My Zsh interactive startup can hang scripted `zsh -ic` checks; disable auto-update or avoid interactive shells.
* `make test` can fail on config-sensitive tests (OpenAI base URL expectations) and Seedream attachment fixtures.
* GUI editors may fail to install `gopls` if `go` is not on their PATH; set launchd env vars or editor tool envs.
* Auth module can be disabled when `AUTH_DATABASE_URL` points to localhost without a running Postgres; start `auth-db` via Docker or clear auth DB envs for in-memory auth.
* Local auth DB setup can skip migrations if `psql` is missing; use Docker exec inside `alex-auth-db` or install `psql`.
