#!/bin/bash
set -e

echo "Copying binaries to npm packages..."

# The root of the repository
ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
BUILD_DIR="$ROOT_DIR/build"
NPM_DIR="$ROOT_DIR/npm"

PLATFORMS=(
  "linux-amd64"
  "linux-arm64"
  "darwin-amd64"
  "darwin-arm64"
  "windows-amd64"
)

for platform in "${PLATFORMS[@]}"; do
  NPM_PKG_DIR="$NPM_DIR/alex-$platform"
  NPM_BIN_DIR="$NPM_PKG_DIR/bin"

  if [[ "$platform" == "windows-amd64" ]]; then
    SRC_BIN="$BUILD_DIR/alex-$platform.exe"
    DEST_BIN="$NPM_BIN_DIR/alex.exe"
    echo "Copying $SRC_BIN to $DEST_BIN"
    cp "$SRC_BIN" "$DEST_BIN"
  else
    SRC_BIN="$BUILD_DIR/alex-$platform"
    DEST_BIN="$NPM_BIN_DIR/alex"
    echo "Copying $SRC_BIN to $DEST_BIN"
    cp "$SRC_BIN" "$DEST_BIN"
  fi
done

echo "Successfully copied all binaries."
