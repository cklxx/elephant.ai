#!/bin/bash
# Remove strict error handling that's causing issues in CI
set -eo pipefail

# Function to safely handle command failures
safe_execute() {
    if ! "$@"; then
        log_error "Command failed: $*"
        return 1
    fi
    return 0
}

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly CYAN='\033[0;36m'
readonly NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $*"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $*"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $*" >&2
}

log_step() {
    echo -e "${CYAN}[STEP]${NC} $*"
}

# Cleanup function for error handling
cleanup() {
    local exit_code=$?
    if [[ $exit_code -ne 0 ]]; then
        log_error "Script failed with exit code $exit_code. Check the output above for details."
    fi
}
trap cleanup EXIT

log_info "Starting NPM binary copy process..."

# The root of the repository
readonly ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
readonly BUILD_DIR="$ROOT_DIR/build"
readonly NPM_DIR="$ROOT_DIR/npm"

log_info "Project root: $ROOT_DIR"
log_info "Build directory: $BUILD_DIR"
log_info "NPM packages directory: $NPM_DIR"

# Platform configurations
readonly PLATFORMS=(
  "linux-amd64"
  "linux-arm64"  
  "darwin-amd64"
  "darwin-arm64"
  "windows-amd64"
)

# Verify build directory exists
if [[ ! -d "$BUILD_DIR" ]]; then
    log_error "Build directory not found: $BUILD_DIR"
    log_error "Please run 'make build-all' first to generate binaries"
    exit 1
fi

# Verify NPM directory exists
if [[ ! -d "$NPM_DIR" ]]; then
    log_error "NPM packages directory not found: $NPM_DIR"
    exit 1
fi

log_info "Found ${#PLATFORMS[@]} platforms to process"

# Track copy results
declare -i success_count=0
declare -i failed_count=0
declare -a failed_platforms=()

# Process each platform
for platform in "${PLATFORMS[@]}"; do
    log_step "Processing platform: $platform"
    
    # NPM package directory (keeping alex-* for directory names)
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
    
    # Validate source binary exists
    if [[ ! -f "$SRC_BIN" ]]; then
        log_error "Source binary not found: $SRC_BIN"
        ((failed_count++))
        failed_platforms+=("$platform")
        continue
    fi
    
    # Validate NPM package directory exists
    if [[ ! -d "$NPM_PKG_DIR" ]]; then
        log_warning "NPM package directory not found: $NPM_PKG_DIR"
        log_warning "Skipping platform: $platform"
        ((failed_count++))
        failed_platforms+=("$platform")
        continue
    fi
    
    # Create bin directory if it doesn't exist
    if [[ ! -d "$NPM_BIN_DIR" ]]; then
        log_info "Creating bin directory: $NPM_BIN_DIR"
        if ! mkdir -p "$NPM_BIN_DIR"; then
            log_error "Failed to create bin directory: $NPM_BIN_DIR"
            ((failed_count++))
            failed_platforms+=("$platform")
            continue
        fi
    fi
    
    # Get source file size for verification (handle different stat formats)
    src_size="unknown"
    if command -v stat >/dev/null 2>&1; then
        # Try GNU stat first (Linux)
        src_size=$(stat -c%s "$SRC_BIN" 2>/dev/null) || \
        # Try BSD stat (macOS)
        src_size=$(stat -f%z "$SRC_BIN" 2>/dev/null) || \
        src_size="unknown"
    fi
    
    log_info "Copying: $(basename "$SRC_BIN") (${src_size} bytes)"
    log_info "From: $SRC_BIN"
    log_info "To: $DEST_BIN"
    
    # Copy binary with error checking
    if cp "$SRC_BIN" "$DEST_BIN"; then
        # Set executable permissions (crucial for binaries)
        if chmod +x "$DEST_BIN"; then
            # Verify copy was successful
            if [[ -f "$DEST_BIN" ]]; then
                dest_size="unknown"
                if command -v stat >/dev/null 2>&1; then
                    # Try GNU stat first (Linux)
                    dest_size=$(stat -c%s "$DEST_BIN" 2>/dev/null) || \
                    # Try BSD stat (macOS) 
                    dest_size=$(stat -f%z "$DEST_BIN" 2>/dev/null) || \
                    dest_size="unknown"
                fi
                
                if [[ "$src_size" == "$dest_size" ]] || [[ "$src_size" == "unknown" ]] || [[ "$dest_size" == "unknown" ]]; then
                    log_success "✓ Successfully copied and set permissions for $platform"
                    ((success_count++))
                else
                    log_error "✗ Size mismatch after copy for $platform (src: $src_size, dest: $dest_size)"
                    ((failed_count++))
                    failed_platforms+=("$platform")
                fi
            else
                log_error "✗ Destination file not found after copy for $platform"
                ((failed_count++))
                failed_platforms+=("$platform")
            fi
        else
            log_error "✗ Failed to set executable permissions for $platform"
            ((failed_count++))
            failed_platforms+=("$platform")
        fi
    else
        log_error "✗ Failed to copy binary for $platform"
        ((failed_count++))
        failed_platforms+=("$platform")
    fi
    
    echo  # Add blank line for readability
done

# Final summary
echo "=========================================="
log_info "Copy process completed"
log_info "Total platforms: ${#PLATFORMS[@]}"
log_success "Successful copies: $success_count"

if [[ $failed_count -gt 0 ]]; then
    log_error "Failed copies: $failed_count"
    log_error "Failed platforms: ${failed_platforms[*]}"
    echo
    log_error "Some binaries failed to copy. Please check the errors above."
    exit 1
else
    log_success "All binaries copied successfully!"
fi

log_info "NPM packages are ready for publishing"
echo "=========================================="
