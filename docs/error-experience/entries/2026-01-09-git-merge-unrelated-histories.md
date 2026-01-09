# 2026-01-09 - git merge unrelated histories

- Error: `git merge origin/main` failed with "refusing to merge unrelated histories" after `origin/main` was force-updated.
- Remediation: use `git merge --allow-unrelated-histories origin/main` after confirming the history split is intended, or rebase/cherry-pick onto the new `origin/main`.
