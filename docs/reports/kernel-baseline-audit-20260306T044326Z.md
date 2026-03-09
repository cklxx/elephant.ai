# Kernel Baseline Audit

- Timestamp (UTC): 20260306T044326Z
- Repo: \/Users\/bytedance\/code\/elephant.ai
- HEAD: 2c9bad23

## Commands Executed
-   git status --short --branch
-   git rev-parse --short HEAD
-   go list ./... | rg 'internal/(infra/(teamruntime|kernel|lark)|app/agent|delivery/channels/lark|infra/tools/builtin/larktools)'
-   rg -n '^package ' internal/infra/lark internal/infra/tools/builtin/larktools internal/delivery/channels/lark internal/app/agent internal/infra/kernel internal/infra/teamruntime
-   go test ./internal/infra/teamruntime/... ./internal/app/agent/... ./internal/infra/kernel/...
-   go test ./internal/infra/lark/... ./internal/infra/tools/builtin/larktools ./internal/delivery/channels/lark/...
-   /Users/bytedance/go/bin/golangci-lint run ./internal/infra/teamruntime/... ./internal/app/agent/... ./internal/infra/kernel/... ./internal/infra/lark/... ./internal/infra/tools/builtin/larktools ./internal/delivery/channels/lark/...

## Verified Effective Package Surface
- Core baseline:
  - internal/infra/teamruntime
  - internal/app/agent/*
  - internal/infra/kernel
- Lark-related verified paths:
  - internal/infra/lark and subpackages
  - internal/infra/tools/builtin/larktools
  - internal/delivery/channels/lark and subpackages

## Results
- Baseline tests (core): PASS
- Baseline tests (lark): PASS
- Minimal lint on relevant packages: PASS

## Evidence Files
- /tmp/kernel_audit_status_20260306T044326Z.log
- /tmp/kernel_audit_packages_20260306T044326Z.log
- /tmp/kernel_audit_test_core_20260306T044326Z.log
- /tmp/kernel_audit_test_lark_20260306T044326Z.log
- /tmp/kernel_audit_lint_20260306T044326Z.log

## STATE.md Write-back Notes
- Audited commit: 2c9bad23
- Revalidated deterministic package baseline for kernel + teamruntime + app/agent + lark surface
- Revalidated lark package surface currently resolves to: internal/infra/lark, internal/infra/tools/builtin/larktools, internal/delivery/channels/lark
- Deterministic baseline tests and minimal lint passed on the audited package set
- No build logic modified in this audit; safe to use as comparison baseline for build-executor repair work

## Notes
- Audit intentionally avoided build logic changes.
- This report is meant to serve as a fresh verification checkpoint for parallel build-executor remediation.
