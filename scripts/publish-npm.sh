#!/bin/bash
set -e

echo "Starting npm publish process..."

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
NPM_DIR="$ROOT_DIR/npm"

# First, run the copy script to ensure binaries are up-to-date
"$ROOT_DIR/scripts/copy-npm-binaries.sh"

PLATFORM_PACKAGES=(
  "alex-linux-amd64"
  "alex-linux-arm64"
  "alex-darwin-amd64"
  "alex-darwin-arm64"
  "alex-windows-amd64"
)

# Publish platform-specific packages first
for pkg in "${PLATFORM_PACKAGES[@]}"; do
  echo "--- Publishing $pkg ---"
  cd "$NPM_DIR/$pkg"
  npm publish --access public
  echo "--- Successfully published $pkg ---"
done

# Publish the main package last
echo "--- Publishing main alex-code package ---"
cd "$NPM_DIR/alex-code"
npm publish --access public
echo "--- Successfully published alex-code ---"

echo "All npm packages have been published successfully!"
