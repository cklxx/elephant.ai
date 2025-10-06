#!/usr/bin/env bash
set -euo pipefail

GO_BIN="${GO_BIN:-$(command -v go || true)}"
if [[ -z "$GO_BIN" ]]; then
  echo "[go-with-toolchain] go executable not found in PATH" >&2
  exit 1
fi

version_str="$($GO_BIN env GOVERSION 2>/dev/null || $GO_BIN version | awk '{print $3}')"
version_str="${version_str#go}"
if [[ "$version_str" =~ ^([0-9]+)\.([0-9]+) ]]; then
  major="${BASH_REMATCH[1]}"
  minor="${BASH_REMATCH[2]}"
  if (( major < 1 || (major == 1 && minor < 21) )); then
    echo "[go-with-toolchain] Go 1.21 or newer is recommended (found ${version_str}). Attempting to continue..." >&2
  fi
fi

# Ensure golang.org/toolchain checksum verification can succeed
original_gosumdb="${GOSUMDB:-}"
if [[ "${GOSUMDB:-}" == "off" ]]; then
  echo "[go-with-toolchain] GOSUMDB=off detected - temporarily enabling sum.golang.org for toolchain verification" >&2
  export GOSUMDB="sum.golang.org"
fi

# Always allow golang.org/toolchain to bypass private module rules
if [[ -n "${GONOSUMDB:-}" ]]; then
  case "${GONOSUMDB}" in
    *golang.org/toolchain*) ;; # already present
    *) export GONOSUMDB="${GONOSUMDB},golang.org/toolchain" ;;
  esac
else
  export GONOSUMDB="golang.org/toolchain"
fi

"$GO_BIN" "$@"
status=$?

if [[ "$original_gosumdb" == "off" ]]; then
  export GOSUMDB="off"
fi

exit $status
