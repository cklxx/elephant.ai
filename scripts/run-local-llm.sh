#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MODEL_PATH="${ROOT_DIR}/models/functiongemma/functiongemma-270m-it-BF16.gguf"
TEMPLATE_PATH="${ROOT_DIR}/models/functiongemma/chat_template.jinja"
LLAMA_SERVER_BIN="${LLAMA_SERVER_BIN:-llama-server}"

if ! command -v "${LLAMA_SERVER_BIN}" >/dev/null 2>&1; then
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  arch="$(uname -m)"
  case "${arch}" in
    arm64|aarch64) arch="arm64" ;;
    x86_64) arch="amd64" ;;
  esac
  fallback="${ROOT_DIR}/.toolchains/llama.cpp/b7658/${os}-${arch}/llama-server"
  if [[ -x "${fallback}" ]]; then
    LLAMA_SERVER_BIN="${fallback}"
  fi
fi

if ! command -v "${LLAMA_SERVER_BIN}" >/dev/null 2>&1; then
  echo "llama-server not found. Run ./alex once to auto-download, or install llama.cpp and ensure llama-server is on PATH."
  exit 1
fi

if [[ ! -f "${MODEL_PATH}" ]]; then
  echo "Model file missing: ${MODEL_PATH}"
  echo "Run: git lfs pull"
  exit 1
fi

if [[ ! -f "${TEMPLATE_PATH}" ]]; then
  echo "Chat template missing: ${TEMPLATE_PATH}"
  exit 1
fi

exec "${LLAMA_SERVER_BIN}" \
  --host 127.0.0.1 \
  --port 11437 \
  -m "${MODEL_PATH}" \
  -c 8192 \
  --jinja \
  --chat-template-file "${TEMPLATE_PATH}"
