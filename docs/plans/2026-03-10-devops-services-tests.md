# Devops Services Lifecycle Test Plan

Date: 2026-03-10

Scope:
- add `internal/devops/services/services_test.go`
- improve lifecycle coverage for backend and web services

Plan:
1. Reuse the real `process.Manager`, `port.Allocator`, and `health.Checker`.
2. Use the test binary itself as a helper child process that serves `/health`.
3. Test backend `Build` with a fake `go-with-toolchain.sh` script that writes an executable staging artifact.
4. Test backend `Start` and `Stop` with a copied helper binary and the real process manager.
5. Test web `NewWebService`, `Start`, and `Stop` with a fake `npm` command that launches the helper server.
6. Test orphan web-process cleanup with a real orphan `npm --prefix <webdir> run dev` process.
7. Run focused `go test`, then mandatory code review, commit, rebase/merge, and remove the worktree.
