#!/bin/bash

# Alex CLI Tool Installation Script
# This script detects the OS and architecture, downloads the appropriate binary,
# and installs it to the user's PATH.

set -e

# Check if we're running under bash, if not, try to re-exec with bash
if [ -z "$BASH_VERSION" ]; then
    # Check if this is a piped execution (common case: curl | sh)
    if [ ! -t 0 ]; then
        # Script is being piped, we can't re-exec, so continue with POSIX-compatible mode
        echo "Info: Running in POSIX-compatible mode (piped execution detected)"
    elif [ -f "$0" ] && command -v bash >/dev/null 2>&1; then
        # Script is a file and bash is available, re-execute with bash
        exec bash "$0" "$@"
    else
        # Continue with current shell
        echo "Info: Continuing with POSIX-compatible shell"
    fi
fi

# 配置变量
BINARY_NAME="alex"
GITHUB_REPO="cklxx/Alex-Code"
INSTALL_DIR="$HOME/.local/bin"
TMP_DIR=""
CHECKSUM_ASSET_NAME="${ALEX_CHECKSUM_ASSET_NAME:-checksums.txt}"

LOGGING_HELPER_LOADED=0
if [ -n "${BASH_VERSION:-}" ]; then
    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" >/dev/null 2>&1 && pwd)"
    LOGGING_HELPER_PATH="${SCRIPT_DIR}/lib/common/logging.sh"
    if [ -f "$LOGGING_HELPER_PATH" ] && . "$LOGGING_HELPER_PATH"; then
        LOGGING_HELPER_LOADED=1
    fi
fi

if [ "$LOGGING_HELPER_LOADED" = "1" ] && ! command -v log_warning >/dev/null 2>&1 && command -v log_warn >/dev/null 2>&1; then
    log_warning() {
        log_warn "$@"
    }
fi

if ! command -v log_info >/dev/null 2>&1; then
    # 颜色输出 (fallback)
    RED='\033[0;31m'
    GREEN='\033[0;32m'
    YELLOW='\033[1;33m'
    BLUE='\033[0;34m'
    NC='\033[0m' # No Color

    # 日志函数 - 检测是否支持颜色
    supports_color() {
        # 检查是否是交互式终端以及TERM变量
        [ -t 1 ] && [ -n "$TERM" ] && [ "$TERM" != "dumb" ]
    }

    log_info() {
        if supports_color; then
            printf "${BLUE}[INFO]${NC} %s\n" "$1"
        else
            printf "[INFO] %s\n" "$1"
        fi
    }

    log_success() {
        if supports_color; then
            printf "${GREEN}[SUCCESS]${NC} %s\n" "$1"
        else
            printf "[SUCCESS] %s\n" "$1"
        fi
    }

    log_warning() {
        if supports_color; then
            printf "${YELLOW}[WARNING]${NC} %s\n" "$1"
        else
            printf "[WARNING] %s\n" "$1"
        fi
    }

    log_error() {
        if supports_color; then
            printf "${RED}[ERROR]${NC} %s\n" "$1"
        else
            printf "[ERROR] %s\n" "$1"
        fi
    }
fi

# 检查命令是否存在 (fallback)
if ! command -v command_exists >/dev/null 2>&1; then
    command_exists() {
        command -v "$1" >/dev/null 2>&1
    }
fi

cleanup_tmp_dir() {
    if [ -n "${TMP_DIR:-}" ] && [ -d "$TMP_DIR" ]; then
        rm -rf "$TMP_DIR"
    fi
}

create_tmp_dir() {
    if ! command_exists mktemp; then
        log_error "mktemp is required to create a secure temporary directory"
        exit 1
    fi

    TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/alex-install.XXXXXX")
    chmod 700 "$TMP_DIR"
}

trap cleanup_tmp_dir EXIT

# 检测操作系统和架构
detect_platform() {
    local os
    local arch
    
    # 检测操作系统
    case "$(uname -s)" in
        Linux*)
            os="linux"
            ;;
        Darwin*)
            os="darwin"
            ;;
        CYGWIN*|MINGW*|MSYS*)
            os="windows"
            ;;
        *)
            log_error "Unsupported operating system: $(uname -s)"
            exit 1
            ;;
    esac
    
    # 检测架构
    case "$(uname -m)" in
        x86_64|amd64)
            arch="amd64"
            ;;
        arm64|aarch64)
            arch="arm64"
            ;;
        *)
            log_error "Unsupported architecture: $(uname -m)"
            exit 1
            ;;
    esac
    
    echo "${os}-${arch}"
}

# 获取最新版本
get_latest_version() {
    log_info "Fetching latest version..."
    
    local api_url="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"
    local response
    
    log_info "Checking releases at: $api_url"
    
    if command_exists curl; then
        response=$(curl -s --connect-timeout 10 --max-time 30 "$api_url" 2>/dev/null)
        curl_exit_code=$?
        if [ $curl_exit_code -ne 0 ]; then
            log_error "Failed to connect to GitHub API (curl exit code: $curl_exit_code)"
            log_info "You can specify a version manually with --version flag"
            exit 1
        fi
        # Check if response indicates no releases found
        if echo "$response" | grep -q '"message".*"Not Found"' 2>/dev/null; then
            log_error "No releases found for repository ${GITHUB_REPO}"
            log_info "This repository doesn't have pre-built releases available."
            log_info "To install Alex, please build from source:"
            log_info "1. git clone https://github.com/${GITHUB_REPO}.git"
            log_info "2. cd Alex-Code"
            log_info "3. make build"
            log_info "4. sudo cp alex /usr/local/bin/"
            exit 1
        fi
    elif command_exists wget; then
        response=$(wget -qO- --timeout=30 --tries=2 "$api_url" 2>/dev/null)
        wget_exit_code=$?
        if [ $wget_exit_code -ne 0 ]; then
            log_error "Failed to connect to GitHub API (wget exit code: $wget_exit_code)"
            log_info "You can specify a version manually with --version flag"
            exit 1
        fi
        # Check if response indicates no releases found
        if echo "$response" | grep -q '"message".*"Not Found"' 2>/dev/null; then
            log_error "No releases found for repository ${GITHUB_REPO}"
            log_info "This repository doesn't have pre-built releases available."
            log_info "To install Alex, please build from source:"
            log_info "1. git clone https://github.com/${GITHUB_REPO}.git"
            log_info "2. cd Alex-Code"
            log_info "3. make build"
            log_info "4. sudo cp alex /usr/local/bin/"
            exit 1
        fi
    else
        log_error "Neither curl nor wget is available. Please install one of them and try again."
        exit 1
    fi
    
    # 更robust的JSON解析
    latest_version=$(echo "$response" | grep '"tag_name"' 2>/dev/null | head -n1 | sed 's/.*"tag_name".*:.*"\([^"]*\)".*/\1/' 2>/dev/null || echo "")
    
    if [ -z "$latest_version" ]; then
        log_error "Failed to parse latest version from GitHub API response"
        log_info "The API response might be rate-limited or malformed"
        log_info "You can specify a version manually with --version flag"
        exit 1
    fi
    
    echo "$latest_version"
}

# 下载文件
download_file() {
    local url="$1"
    local output="$2"
    
    log_info "Downloading from: $url"
    
    if command_exists curl; then
        if ! curl -sSfL --connect-timeout 10 --max-time 300 "$url" -o "$output"; then
            log_error "Download failed with curl"
            return 1
        fi
    elif command_exists wget; then
        if ! wget -q --timeout=300 --tries=3 "$url" -O "$output"; then
            log_error "Download failed with wget"
            return 1
        fi
    else
        log_error "Neither curl nor wget is available. Please install one of them and try again."
        exit 1
    fi
    
    # 验证文件是否下载成功
    if [ ! -f "$output" ] || [ ! -s "$output" ]; then
        log_error "Downloaded file is empty or missing: $output"
        return 1
    fi
    
    return 0
}

download_file_optional() {
    local url="$1"
    local output="$2"

    if command_exists curl; then
        curl -sSfL --connect-timeout 10 --max-time 120 "$url" -o "$output" >/dev/null 2>&1
        return $?
    elif command_exists wget; then
        wget -q --timeout=120 --tries=2 "$url" -O "$output" >/dev/null 2>&1
        return $?
    fi

    return 1
}

compute_sha256() {
    local file_path="$1"

    if command_exists sha256sum; then
        sha256sum "$file_path" | awk '{print $1}'
        return 0
    fi

    if command_exists shasum; then
        shasum -a 256 "$file_path" | awk '{print $1}'
        return 0
    fi

    return 1
}

verify_sha256_checksum() {
    local file_path="$1"
    local expected_checksum="$2"
    local artifact_name="$3"
    local require_verification="${4:-0}"
    local actual_checksum
    local expected_normalized
    local actual_normalized

    if ! actual_checksum=$(compute_sha256 "$file_path"); then
        if [ "$require_verification" = "1" ]; then
            log_error "sha256 tool not found; cannot verify checksum for $artifact_name"
            return 1
        fi
        log_warning "sha256 tool not found; skipping checksum verification for $artifact_name"
        return 0
    fi

    expected_normalized=$(printf '%s' "$expected_checksum" | tr '[:upper:]' '[:lower:]')
    actual_normalized=$(printf '%s' "$actual_checksum" | tr '[:upper:]' '[:lower:]')

    if [ "$actual_normalized" != "$expected_normalized" ]; then
        log_error "Checksum mismatch for $artifact_name"
        return 1
    fi

    log_success "Checksum verified for $artifact_name"
    return 0
}

verify_release_checksum() {
    local binary_path="$1"
    local version="$2"
    local binary_name="$3"
    local expected_checksum="${ALEX_INSTALL_CHECKSUM:-}"
    local checksum_url
    local checksum_file

    if [ "${ALEX_INSTALL_SKIP_CHECKSUM:-0}" = "1" ]; then
        log_warning "Skipping checksum verification (ALEX_INSTALL_SKIP_CHECKSUM=1)"
        return 0
    fi

    if [ -n "$expected_checksum" ]; then
        verify_sha256_checksum "$binary_path" "$expected_checksum" "$binary_name" 1
        return $?
    fi

    checksum_url="${ALEX_INSTALL_CHECKSUM_URL:-https://github.com/${GITHUB_REPO}/releases/download/${version}/${CHECKSUM_ASSET_NAME}}"
    checksum_file="$TMP_DIR/${CHECKSUM_ASSET_NAME##*/}"

    if ! download_file_optional "$checksum_url" "$checksum_file"; then
        log_warning "Checksum file unavailable at $checksum_url; continuing without checksum verification"
        return 0
    fi

    expected_checksum=$(awk -v target="$binary_name" '
        $1 ~ /^[[:xdigit:]]+$/ {
            file=$2
            gsub(/^\*/, "", file)
            if (file == target) {
                print $1
                exit
            }
        }
    ' "$checksum_file")

    if [ -z "$expected_checksum" ]; then
        log_warning "No checksum entry for $binary_name in $checksum_url; continuing without checksum verification"
        return 0
    fi

    verify_sha256_checksum "$binary_path" "$expected_checksum" "$binary_name" 0
}

# 验证下载的文件
verify_binary() {
    local binary_path="$1"
    
    if [ ! -f "$binary_path" ]; then
        log_error "Downloaded binary not found: $binary_path"
        return 1
    fi
    
    if [ ! -x "$binary_path" ]; then
        log_error "Downloaded binary is not executable: $binary_path"
        return 1
    fi
    
    # 尝试运行 version 检查
    if ! "$binary_path" version >/dev/null 2>&1; then
        log_warning "Binary may not be working correctly (version command failed)"
        return 1
    fi
    
    return 0
}

# 安装到系统
install_binary() {
    local binary_path="$1"
    local platform="$2"
    
    # 确保安装目录存在
    mkdir -p "$INSTALL_DIR"
    
    # 复制二进制文件
    local target_path="$INSTALL_DIR/$BINARY_NAME"
    cp "$binary_path" "$target_path"
    chmod +x "$target_path"
    
    log_success "Binary installed to: $target_path"
    
    # 检查PATH
    if ! echo ":$PATH:" | grep -q ":$INSTALL_DIR:"; then
        log_warning "$INSTALL_DIR is not in your PATH"
        log_info "Add the following line to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        echo "export PATH=\"\$PATH:$INSTALL_DIR\""
        echo ""
        log_info "Or run the following command to add it temporarily:"
        echo "export PATH=\"\$PATH:$INSTALL_DIR\""
        echo ""
    fi
    
    # 验证安装
    if command_exists "$BINARY_NAME"; then
        log_success "Installation successful! You can now use '$BINARY_NAME'"
        "$BINARY_NAME" version
    else
        log_warning "Installation completed, but '$BINARY_NAME' is not found in PATH"
        log_info "You may need to restart your shell or update your PATH"
    fi
}

# 安装系统依赖
install_dependencies() {
    log_info "Installing system dependencies..."
    
    # 检测并安装ripgrep
    if ! command_exists rg; then
        log_info "Installing ripgrep..."
        case "$(uname -s)" in
            Darwin*)
                if command_exists brew; then
                    if brew install ripgrep; then
                        log_success "ripgrep installed via Homebrew"
                    else
                        log_warning "Failed to install ripgrep via Homebrew"
                    fi
                elif command_exists port; then
                    if sudo port install ripgrep; then
                        log_success "ripgrep installed via MacPorts"
                    else
                        log_warning "Failed to install ripgrep via MacPorts"
                    fi
                else
                    log_warning "Neither brew nor port found. Please install ripgrep manually."
                    log_info "Install Homebrew: /bin/bash -c \"\$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)\""
                fi
                ;;
            Linux*)
                if command_exists apt; then
                    if sudo apt update && sudo apt install -y ripgrep; then
                        log_success "ripgrep installed via apt"
                    else
                        log_warning "Failed to install ripgrep via apt"
                    fi
                elif command_exists dnf; then
                    if sudo dnf install -y ripgrep; then
                        log_success "ripgrep installed via dnf"
                    else
                        log_warning "Failed to install ripgrep via dnf"
                    fi
                elif command_exists yum; then
                    if sudo yum install -y ripgrep; then
                        log_success "ripgrep installed via yum"
                    else
                        log_warning "Failed to install ripgrep via yum"
                    fi
                elif command_exists pacman; then
                    if sudo pacman -S --noconfirm ripgrep; then
                        log_success "ripgrep installed via pacman"
                    else
                        log_warning "Failed to install ripgrep via pacman"
                    fi
                elif command_exists zypper; then
                    if sudo zypper install -y ripgrep; then
                        log_success "ripgrep installed via zypper"
                    else
                        log_warning "Failed to install ripgrep via zypper"
                    fi
                else
                    log_warning "Package manager not found. Please install ripgrep manually."
                    log_info "Download from: https://github.com/BurntSushi/ripgrep/releases"
                fi
                ;;
            *)
                log_warning "Unsupported OS for automatic dependency installation. Please install ripgrep manually."
                ;;
        esac
        
        # 验证安装
        if command_exists rg; then
            log_success "ripgrep installed successfully"
        else
            log_warning "ripgrep installation may have failed"
        fi
    else
        log_info "ripgrep is already installed"
    fi
}

# 主安装流程
main() {
    log_info "Starting Alex CLI installation..."
    
    # 安装依赖
    install_dependencies
    
    # 检测平台
    platform=$(detect_platform)
    log_info "Detected platform: $platform"
    
    # 获取版本
    if [ -n "$VERSION" ]; then
        version="$VERSION"
        log_info "Using specified version: $version"
    elif [ "$TEST_MODE" = "1" ]; then
        version="v1.0.0"
        log_info "Test mode - using version: $version"
    else
        version=$(get_latest_version)
        log_info "Latest version: $version"
    fi
    
    # 构建下载URL
    binary_suffix=""
    case "$platform" in
        windows-*)
            binary_suffix=".exe"
            ;;
    esac
    
    binary_name="${BINARY_NAME}-${platform}${binary_suffix}"
    download_url="https://github.com/${GITHUB_REPO}/releases/download/${version}/${binary_name}"
    
    # 创建临时目录
    create_tmp_dir
    
    # 下载二进制文件
    binary_path="$TMP_DIR/$binary_name"
    if ! download_file "$download_url" "$binary_path"; then
        log_error "Failed to download binary from: $download_url"
        log_info "Please check:"
        log_info "1. Your internet connection"
        log_info "2. The release exists at: https://github.com/${GITHUB_REPO}/releases"
        log_info "3. Try specifying a different version with --version flag"
        exit 1
    fi

    if ! verify_release_checksum "$binary_path" "$version" "$binary_name"; then
        log_error "Binary checksum verification failed"
        exit 1
    fi
    
    # 使二进制文件可执行
    chmod +x "$binary_path"
    
    # 验证二进制文件
    if ! verify_binary "$binary_path"; then
        log_error "Binary verification failed"
        exit 1
    fi
    
    # 安装到系统
    install_binary "$binary_path" "$platform"
    
    log_success "Alex CLI has been successfully installed!"
    log_info "Run 'alex --help' to get started"
}

# 处理脚本参数
while [ $# -gt 0 ]; do
    case $1 in
        --version)
            VERSION="$2"
            shift 2
            ;;
        --repo)
            GITHUB_REPO="$2"
            shift 2
            ;;
        --install-dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --test)
            TEST_MODE=1
            shift
            ;;
        --help)
            echo "Alex CLI Installation Script"
            echo ""
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --version VERSION     Install specific version (default: latest)"
            echo "  --repo REPO          GitHub repository (default: $GITHUB_REPO)"
            echo "  --install-dir DIR    Installation directory (default: $INSTALL_DIR)"
            echo "  --test               Test mode - use v1.0.0 as version"
            echo "  --help               Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                           # Install latest version"
            echo "  $0 --version v1.0.0         # Install specific version"
            echo "  $0 --install-dir /usr/local/bin  # Install to custom directory"
            echo "  $0 --test                   # Test installation with v1.0.0"
            exit 0
            ;;
        *)
            log_error "Unknown option: $1"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

# 运行主函数
main
