# 2026-01-26 - make test transient failure (CJK fullscreen test)

- Summary: `make test` failed once with a non-existent `TestShouldUseFullscreenTUIForcesLineInputForCJKLocale`; rerun passed.
- Remediation: rerun tests; investigate if this reappears (possible cache/output mismatch).
- Resolution: resolved by rerunning `make test`.
