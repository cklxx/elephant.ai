# STATE

Updated: 2026-03-10

## Current status

- All tests pass (14/14 packages). Lint clean.
- `origin/main` synced.
- `CC=/usr/bin/clang` required for CGO builds (Node.js shim at `~/.local/bin/cc` breaks `-E` flag).

## Known issues

| Issue | Status |
|-------|--------|
| `cc` PATH shadowing by Node.js shim | Mitigated — use `CC=/usr/bin/clang` in all test/build invocations |
| larktools/infra/lark architectural split | Accepted — defer unless new features touch both layers |

## Next actions

1. Add `CC=/usr/bin/clang` to Makefile/CI targets to prevent CGO false negatives.
2. Optional: prune stale tmux sessions from historical team runs.
