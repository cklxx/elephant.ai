Summary: `./dev.sh down && ./dev.sh` failed during sandbox startup with `env_flags[@]: unbound variable` in `scripts/lib/common/sandbox.sh`.
Remediation: Guarded env_flags array expansion to avoid unbound access under `set -u`.
