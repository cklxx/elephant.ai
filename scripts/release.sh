#!/bin/bash

# Release helper for the Alex project
# Builds cross-platform binaries, runs checks, and prepares release notes

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

ARTIFACT_DIR="release"
CLI_PACKAGE="./cmd/alex"

export GOMODCACHE="${PROJECT_ROOT}/.cache/go/pkg/mod"
export GOCACHE="${PROJECT_ROOT}/.cache/go/build"
mkdir -p "${GOMODCACHE}" "${GOCACHE}"

print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

get_version() {
    if [[ -n "$1" ]]; then
        echo "$1"
    elif git describe --tags --exact-match HEAD >/dev/null 2>&1; then
        git describe --tags --exact-match HEAD
    else
        echo "v$(date +%Y%m%d)-$(git rev-parse --short HEAD)"
    fi
}

validate_version() {
    local version="$1"
    if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9._-]+)?$ ]] && [[ ! "$version" =~ ^v[0-9]{8}-[a-f0-9]+$ ]]; then
        print_error "Invalid version format: $version"
        print_error "Expected semantic version (e.g., v1.2.3 or v1.2.3-beta.1)"
        exit 1
    fi
}

ensure_clean_git() {
    if [[ -n "$(git status --porcelain)" ]]; then
        print_error "Git working directory is not clean. Commit or stash changes before releasing."
        exit 1
    fi
}

run_checks() {
    print_status "Running test suite..."
    ./scripts/test.sh all

    print_status "Running golangci-lint..."
    ./scripts/run-golangci-lint.sh run ./...
}

prepare_workspace() {
    rm -rf "${ARTIFACT_DIR}"
    mkdir -p "${ARTIFACT_DIR}"
}

build_for_platform() {
    local version="$1"
    local goos="$2"
    local goarch="$3"

    local archive_name="alex-${version}-${goos}-${goarch}"
    local staging_dir="${ARTIFACT_DIR}/${archive_name}"
    local binary_name="alex"

    if [[ "${goos}" == "windows" ]]; then
        binary_name="alex.exe"
    fi

    mkdir -p "${staging_dir}"

    print_status "Building ${goos}/${goarch}..."
    GOOS="${goos}" GOARCH="${goarch}" go build -ldflags="-w -s" -o "${staging_dir}/${binary_name}" "${CLI_PACKAGE}"

    cp LICENSE "${staging_dir}/"
    cp README.md "${staging_dir}/"

    if [[ "${goos}" == "windows" ]]; then
        if ! command -v zip >/dev/null 2>&1; then
            print_error "zip command not available; install zip to package Windows artifacts."
            exit 1
        fi
        (cd "${ARTIFACT_DIR}" && zip -qr "${archive_name}.zip" "${archive_name}")
    else
        if ! command -v tar >/dev/null 2>&1; then
            print_error "tar command not available; install tar to package Unix artifacts."
            exit 1
        fi
        (cd "${ARTIFACT_DIR}" && tar -czf "${archive_name}.tar.gz" "${archive_name}")
    fi

    rm -rf "${staging_dir}"
}

build_release() {
    local version="$1"
    local platforms=(
        "linux/amd64"
        "linux/arm64"
        "darwin/amd64"
        "darwin/arm64"
        "windows/amd64"
        "windows/arm64"
    )

    for platform in "${platforms[@]}"; do
        IFS="/" read -r goos goarch <<< "${platform}"
        build_for_platform "${version}" "${goos}" "${goarch}"
    done
}

generate_checksums() {
    print_status "Generating SHA256 checksums..."
    if command -v sha256sum >/dev/null 2>&1; then
        (cd "${ARTIFACT_DIR}" && sha256sum * > checksums.sha256)
    elif command -v shasum >/dev/null 2>&1; then
        (cd "${ARTIFACT_DIR}" && shasum -a 256 * > checksums.sha256)
    else
        print_warning "Could not find sha256sum/shasum; skipping checksum generation."
    fi
}

create_release_notes() {
    local version="$1"
    local notes_file="${ARTIFACT_DIR}/RELEASE_NOTES_${version}.md"

    cat > "${notes_file}" <<EOF
# Alex ${version}

Alex is a terminal-native AI coding agent with a modern TUI, persistent sessions, and flexible tool integrations.

## Highlights
- Hexagonal architecture with clear domain boundaries
- Streaming terminal UI with session history
- Configurable LLM providers (OpenAI, DeepSeek, OpenRouter, Ollama)
- Code indexing, search, and MCP integration
- Go-based core with fast startup and low memory footprint

## Installation
- Linux/macOS: download the \`alex-${version}-<platform>.tar.gz\` artifact and add the binary to your PATH
- Windows: download \`alex-${version}-windows-<arch>.zip\` and place \`alex.exe\` in a directory on PATH
- Homebrew and other package instructions are available in the README

## Verification
- Checksums are provided in \`checksums.sha256\`
- Run \`./alex --help\` after installation to verify the CLI

Happy hacking!
EOF
    print_success "Release notes created at ${notes_file}"
}

show_summary() {
    local version="$1"
    print_success "Release artifacts prepared in ${ARTIFACT_DIR}/"
    ls -1 "${ARTIFACT_DIR}"
    echo ""
    print_status "Next steps:"
    echo "- Create a Git tag: git tag ${version} && git push origin ${version}"
    echo "- Draft a GitHub release and upload artifacts from ${ARTIFACT_DIR}/"
}

main() {
    local version
    version="$(get_version "$1")"
    validate_version "${version}"
    ensure_clean_git
    run_checks
    prepare_workspace
    build_release "${version}"
    generate_checksums
    create_release_notes "${version}"
    show_summary "${version}"
}

main "$@"
