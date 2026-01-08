#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODEL_PATH="${ROOT_DIR}/models/functiongemma/functiongemma-270m-it-BF16.gguf"
TEMPLATE_PATH="${ROOT_DIR}/models/functiongemma/chat_template.jinja"
MODEL_ALIAS="functiongemma-270m-it"
MODEL_URL="https://huggingface.co/unsloth/functiongemma-270m-it-GGUF/resolve/main/functiongemma-270m-it-BF16.gguf"
LLAMA_SERVER_BIN="${LLAMA_SERVER_BIN:-llama-server}"
LLAMA_RELEASE="b7658"

download_file() {
  local url="$1"
  local dest="$2"

  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${url}" -o "${dest}"
    return 0
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -q "${url}" -O "${dest}"
    return 0
  fi
  echo "curl or wget is required to download ${url}"
  return 1
}

download_with_fallback() {
  local dest="$1"
  shift

  for url in "$@"; do
    if download_file "${url}" "${dest}"; then
      return 0
    fi
  done
  return 1
}

ensure_llama_server() {
  if command -v "${LLAMA_SERVER_BIN}" >/dev/null 2>&1; then
    return 0
  fi

  local os
  local arch
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "${arch}" in
    arm64|aarch64) arch="arm64" ;;
    x86_64) arch="amd64" ;;
  esac

  local asset
  case "${os}-${arch}" in
    darwin-arm64) asset="llama-${LLAMA_RELEASE}-bin-macos-arm64.tar.gz" ;;
    darwin-amd64) asset="llama-${LLAMA_RELEASE}-bin-macos-x64.tar.gz" ;;
    linux-amd64) asset="llama-${LLAMA_RELEASE}-bin-ubuntu-x64.tar.gz" ;;
    *)
      echo "No prebuilt llama-server for ${os}/${arch}; install llama.cpp and add llama-server to PATH."
      return 1
      ;;
  esac

  local base_dir="${ROOT_DIR}/.toolchains/llama.cpp/${LLAMA_RELEASE}/${os}-${arch}"
  local target="${base_dir}/llama-server"
  if [[ -x "${target}" ]]; then
    LLAMA_SERVER_BIN="${target}"
    return 0
  fi

  mkdir -p "${base_dir}"
  local archive="${base_dir}/${asset}"
  if [[ ! -f "${archive}" ]]; then
    echo "Downloading llama.cpp server (${asset})..."
    download_with_fallback \
      "${archive}" \
      "https://github.com/ggml-org/llama.cpp/releases/download/${LLAMA_RELEASE}/${asset}" \
      "https://github.com/ggerganov/llama.cpp/releases/download/${LLAMA_RELEASE}/${asset}"
  fi

  tar -xzf "${archive}" --strip-components=1 -C "${base_dir}"
  chmod +x "${target}"
  LLAMA_SERVER_BIN="${target}"
}

ensure_llama_server

if [[ ! -f "${MODEL_PATH}" ]]; then
  echo "Downloading FunctionGemma weights..."
  mkdir -p "$(dirname "${MODEL_PATH}")"
  download_file "${MODEL_URL}" "${MODEL_PATH}"
fi

if [[ ! -f "${TEMPLATE_PATH}" ]]; then
  echo "Chat template missing: ${TEMPLATE_PATH}"
  exit 1
fi

exec "${LLAMA_SERVER_BIN}" \
  --host 127.0.0.1 \
  --port 11437 \
  --alias "${MODEL_ALIAS}" \
  -m "${MODEL_PATH}" \
  -c 8192 \
  --jinja \
  --chat-template-file "${TEMPLATE_PATH}"
