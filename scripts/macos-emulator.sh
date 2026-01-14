#!/usr/bin/env bash
set -euo pipefail

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "This script is for macOS only."
  exit 1
fi

log_info() {
  printf '[INFO] %s\n' "$1"
}

log_warn() {
  printf '[WARN] %s\n' "$1"
}

log_error() {
  printf '[ERROR] %s\n' "$1" >&2
}

run_sdkmanager() {
  set +o pipefail
  yes | "${SDKMANAGER}" "$@"
  local sdk_status=${PIPESTATUS[1]}
  set -o pipefail
  if [[ ${sdk_status} -ne 0 ]]; then
    log_error "sdkmanager failed with status ${sdk_status}"
    exit "${sdk_status}"
  fi
}

ANDROID_SDK_ROOT="${ANDROID_SDK_ROOT:-${ANDROID_HOME:-$HOME/Library/Android/sdk}}"
AVD_NAME="${AVD_NAME:-elephant_ai}"
ANDROID_API_LEVEL="${ANDROID_API_LEVEL:-34}"
ANDROID_SYS_IMG_VARIANT="${ANDROID_SYS_IMG_VARIANT:-google_apis}"
ANDROID_DEVICE="${ANDROID_DEVICE:-pixel}"
EMULATOR_FLAGS="${EMULATOR_FLAGS:-}"

HOST_ARCH="$(uname -m)"
case "${HOST_ARCH}" in
  arm64|aarch64)
    ANDROID_ABI="arm64-v8a"
    ;;
  x86_64|amd64)
    ANDROID_ABI="x86_64"
    ;;
  *)
    log_error "Unsupported architecture: ${HOST_ARCH}"
    exit 1
    ;;
esac

if ! command -v brew >/dev/null 2>&1; then
  log_error "Homebrew is required to install Android command line tools."
  log_error "Install it from https://brew.sh/ and re-run this script."
  exit 1
fi

if ! command -v sdkmanager >/dev/null 2>&1; then
  log_info "Installing Android command line tools (brew cask android-commandlinetools)..."
  brew install --cask android-commandlinetools
fi

if ! command -v avdmanager >/dev/null 2>&1; then
  log_error "avdmanager not found after installing command line tools."
  exit 1
fi

if ! command -v java >/dev/null 2>&1; then
  log_info "Installing Java (Temurin) for SDK tooling..."
  brew install --cask temurin
fi

mkdir -p "${ANDROID_SDK_ROOT}"
export ANDROID_SDK_ROOT
export ANDROID_HOME="${ANDROID_SDK_ROOT}"

SDKMANAGER="$(command -v sdkmanager)"
AVDMANAGER="$(command -v avdmanager)"

log_info "Using Android SDK root: ${ANDROID_SDK_ROOT}"

SDK_PACKAGES_READY=1
if [[ ! -x "${ANDROID_SDK_ROOT}/emulator/emulator" ]]; then
  SDK_PACKAGES_READY=0
fi
if [[ ! -d "${ANDROID_SDK_ROOT}/platform-tools" ]]; then
  SDK_PACKAGES_READY=0
fi
if [[ ! -d "${ANDROID_SDK_ROOT}/platforms/android-${ANDROID_API_LEVEL}" ]]; then
  SDK_PACKAGES_READY=0
fi
if [[ ! -d "${ANDROID_SDK_ROOT}/system-images/android-${ANDROID_API_LEVEL}/${ANDROID_SYS_IMG_VARIANT}/${ANDROID_ABI}" ]]; then
  SDK_PACKAGES_READY=0
fi

if [[ "${SDK_PACKAGES_READY}" -eq 1 ]]; then
  log_info "Android SDK packages already installed; skipping install."
else
  log_info "Ensuring Android SDK packages are installed..."
  run_sdkmanager --sdk_root="${ANDROID_SDK_ROOT}" \
    "platform-tools" \
    "emulator" \
    "platforms;android-${ANDROID_API_LEVEL}" \
    "system-images;android-${ANDROID_API_LEVEL};${ANDROID_SYS_IMG_VARIANT};${ANDROID_ABI}"

  run_sdkmanager --sdk_root="${ANDROID_SDK_ROOT}" --licenses >/dev/null
fi

EMULATOR_BIN="${ANDROID_SDK_ROOT}/emulator/emulator"
if [[ ! -x "${EMULATOR_BIN}" ]]; then
  if command -v emulator >/dev/null 2>&1; then
    EMULATOR_BIN="$(command -v emulator)"
  fi
fi

if [[ ! -x "${EMULATOR_BIN}" ]]; then
  log_error "Android emulator binary not found after SDK install."
  exit 1
fi

if "${EMULATOR_BIN}" -list-avds | grep -qx "${AVD_NAME}"; then
  log_info "AVD already exists: ${AVD_NAME}"
else
  log_info "Creating AVD: ${AVD_NAME}"
  printf 'no\n' | "${AVDMANAGER}" create avd \
    -n "${AVD_NAME}" \
    -k "system-images;android-${ANDROID_API_LEVEL};${ANDROID_SYS_IMG_VARIANT};${ANDROID_ABI}" \
    -d "${ANDROID_DEVICE}"
fi

if pgrep -f "emulator.*-avd ${AVD_NAME}" >/dev/null 2>&1; then
  log_info "Emulator already running for AVD ${AVD_NAME}"
  exit 0
fi

log_info "Starting emulator for AVD ${AVD_NAME}..."
EMULATOR_LOG="${ANDROID_SDK_ROOT}/emulator-${AVD_NAME}.log"

if [[ -n "${EMULATOR_FLAGS}" ]]; then
  nohup "${EMULATOR_BIN}" -avd "${AVD_NAME}" ${EMULATOR_FLAGS} >"${EMULATOR_LOG}" 2>&1 &
else
  nohup "${EMULATOR_BIN}" -avd "${AVD_NAME}" >"${EMULATOR_LOG}" 2>&1 &
fi

log_info "Emulator launch triggered. Log: ${EMULATOR_LOG}"
