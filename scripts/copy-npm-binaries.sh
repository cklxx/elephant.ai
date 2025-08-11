#!/bin/bash

# Simple and reliable NPM binary copy script
# Avoid complex error handling that causes CI issues

echo "[INFO] Starting NPM binary copy process..."

# Basic configuration
ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
BUILD_DIR="$ROOT_DIR/build"
NPM_DIR="$ROOT_DIR/npm"

echo "[INFO] Project root: $ROOT_DIR"
echo "[INFO] Build directory: $BUILD_DIR"
echo "[INFO] NPM packages directory: $NPM_DIR"

# Check directories exist
if [[ ! -d "$BUILD_DIR" ]]; then
    echo "[ERROR] Build directory not found: $BUILD_DIR"
    exit 1
fi

if [[ ! -d "$NPM_DIR" ]]; then
    echo "[ERROR] NPM packages directory not found: $NPM_DIR"
    exit 1
fi

# Platform configurations
PLATFORMS=(
  "linux-amd64"
  "linux-arm64"
  "darwin-amd64"
  "darwin-arm64"
  "windows-amd64"
)

echo "[INFO] Found ${#PLATFORMS[@]} platforms to process"

SUCCESS_COUNT=0
TOTAL_COUNT=${#PLATFORMS[@]}

# Process each platform
for platform in "${PLATFORMS[@]}"; do
    echo "[STEP] Processing platform: $platform"
    
    NPM_PKG_DIR="$NPM_DIR/alex-$platform"
    NPM_BIN_DIR="$NPM_PKG_DIR/bin"
    
    # Determine source and destination paths
    if [[ "$platform" == "windows-amd64" ]]; then
        SRC_BIN="$BUILD_DIR/alex-$platform.exe"
        DEST_BIN="$NPM_BIN_DIR/alex.exe"
    else
        SRC_BIN="$BUILD_DIR/alex-$platform"
        DEST_BIN="$NPM_BIN_DIR/alex"
    fi
    
    # Check source binary exists
    if [[ ! -f "$SRC_BIN" ]]; then
        echo "[ERROR] Source binary not found: $SRC_BIN"
        continue
    fi
    
    # Check NPM package directory exists
    if [[ ! -d "$NPM_PKG_DIR" ]]; then
        echo "[WARNING] NPM package directory not found: $NPM_PKG_DIR"
        continue
    fi
    
    # Create bin directory if needed
    mkdir -p "$NPM_BIN_DIR"
    
    # Get file size for logging
    if command -v stat >/dev/null 2>&1; then
        SRC_SIZE=$(stat -c%s "$SRC_BIN" 2>/dev/null || stat -f%z "$SRC_BIN" 2>/dev/null || echo "unknown")
    else
        SRC_SIZE="unknown"
    fi
    
    echo "[INFO] Copying: $(basename "$SRC_BIN") (${SRC_SIZE} bytes)"
    echo "[INFO] From: $SRC_BIN"
    echo "[INFO] To: $DEST_BIN"
    
    # Copy binary
    if cp "$SRC_BIN" "$DEST_BIN"; then
        # Set executable permissions
        if chmod +x "$DEST_BIN"; then
            echo "[SUCCESS] ✓ Successfully copied and set permissions for $platform"
            SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
        else
            echo "[ERROR] ✗ Failed to set executable permissions for $platform"
        fi
    else
        echo "[ERROR] ✗ Failed to copy binary for $platform"
    fi
    
    echo  # Add blank line for readability
done

# Final summary
echo "=========================================="
echo "[INFO] Copy process completed"
echo "[INFO] Total platforms: $TOTAL_COUNT"
echo "[SUCCESS] Successful copies: $SUCCESS_COUNT"

if [[ $SUCCESS_COUNT -eq $TOTAL_COUNT ]]; then
    echo "[SUCCESS] All binaries copied successfully!"
    echo "[INFO] NPM packages are ready for publishing"
    echo "=========================================="
    exit 0
else
    FAILED_COUNT=$((TOTAL_COUNT - SUCCESS_COUNT))
    echo "[ERROR] Failed copies: $FAILED_COUNT"
    echo "[ERROR] Some binaries failed to copy. Please check the errors above."
    exit 1
fi