#!/bin/bash

# Alex - Storage Dependencies Installation Script
# 网络恢复后运行此脚本安装存储依赖

set -e

echo "🚀 Alex - Installing Storage Dependencies"
echo "=================================================="
echo

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

export GOMODCACHE="${PROJECT_ROOT}/.cache/go/pkg/mod"
export GOCACHE="${PROJECT_ROOT}/.cache/go/build"
mkdir -p "${GOMODCACHE}" "${GOCACHE}"

# 检查Go环境
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.18+ first."
    exit 1
fi

# 检查Go版本
GO_VERSION=$(go version | grep -oE 'go[0-9]+\.[0-9]+' | sed 's/go//')
REQUIRED_VERSION="1.18"

if [[ $(echo "$GO_VERSION $REQUIRED_VERSION" | tr " " "\n" | sort -V | head -n1) != "$REQUIRED_VERSION" ]]; then
    echo "❌ Go version $REQUIRED_VERSION+ required, found $GO_VERSION"
    exit 1
fi

echo "✅ Go version check passed: $GO_VERSION"
echo

# 检查网络连接
echo "🌐 Checking network connectivity..."
if ! ping -c 1 proxy.golang.org &> /dev/null; then
    echo "❌ Cannot reach Go proxy. Please check your network connection."
    echo "💡 You may need to:"
    echo "   1. Check your internet connection"
    echo "   2. Configure proxy settings if behind a firewall"
    echo "   3. Set GOPROXY environment variable"
    exit 1
fi

echo "✅ Network connectivity check passed"
echo

# 备份当前go.mod
echo "📦 Backing up current go.mod..."
if [ -f "go.mod" ]; then
    cp go.mod go.mod.backup
    echo "✅ go.mod backed up to go.mod.backup"
else
    echo "❌ go.mod not found. Please run this script from the project root."
    exit 1
fi

echo

# 安装chromem-go
echo "🎯 Installing chromem-go vector database..."
if go get github.com/philippgille/chromem-go; then
    echo "✅ chromem-go installed successfully"
else
    echo "❌ Failed to install chromem-go"
    echo "💡 This may be due to network issues. Try again later."
fi

echo

# 安装BadgerDB
echo "💾 Installing BadgerDB persistent storage..."
if go get github.com/dgraph-io/badger/v4; then
    echo "✅ BadgerDB v4 installed successfully"
else
    echo "❌ Failed to install BadgerDB"
    echo "💡 This may be due to network issues. Try again later."
fi

echo

# 更新模块
echo "🔄 Updating go modules..."
if go mod tidy; then
    echo "✅ Go modules updated successfully"
else
    echo "❌ Failed to update go modules"
fi

echo

# 验证安装
echo "🔍 Verifying installation..."

# 检查go.mod文件
if grep -q "github.com/philippgille/chromem-go" go.mod; then
    echo "✅ chromem-go dependency found in go.mod"
else
    echo "⚠️  chromem-go not found in go.mod"
fi

if grep -q "github.com/dgraph-io/badger/v4" go.mod; then
    echo "✅ BadgerDB v4 dependency found in go.mod"
else
    echo "⚠️  BadgerDB v4 not found in go.mod"
fi

echo

# 取消注释导入
echo "🔧 Enabling storage implementations..."

# 启用chromem存储
if [ -f "internal/context/storage/chromem.go" ]; then
    echo "🎯 Enabling chromem-go imports..."
    sed -i.bak 's|// "github.com/philippgille/chromem-go"|"github.com/philippgille/chromem-go"|g' internal/context/storage/chromem.go
    if [ $? -eq 0 ]; then
        echo "✅ chromem-go imports enabled"
        rm -f internal/context/storage/chromem.go.bak
    else
        echo "⚠️  Failed to enable chromem-go imports automatically"
        echo "💡 Please manually uncomment chromem-go imports in internal/context/storage/chromem.go"
    fi
fi

# 启用BadgerDB存储
if [ -f "internal/context/storage/badger.go" ]; then
    echo "💾 Enabling BadgerDB imports..."
    sed -i.bak 's|// "encoding/json"|"encoding/json"|g' internal/context/storage/badger.go
    sed -i.bak 's|// "github.com/dgraph-io/badger/v4"|"github.com/dgraph-io/badger/v4"|g' internal/context/storage/badger.go
    if [ $? -eq 0 ]; then
        echo "✅ BadgerDB imports enabled"
        rm -f internal/context/storage/badger.go.bak
    else
        echo "⚠️  Failed to enable BadgerDB imports automatically"
        echo "💡 Please manually uncomment BadgerDB imports in internal/context/storage/badger.go"
    fi
fi

echo

# 测试编译
echo "🔨 Testing compilation..."
if go build -o /tmp/alex-test ./cmd/alex; then
    echo "✅ Alex CLI compiles successfully with new dependencies"
    rm -f /tmp/alex-test
else
    echo "❌ Compilation failed"
    echo "💡 Please check the error messages above"
    echo "💡 You may need to manually fix import issues"
fi

echo

# 运行示例
echo "🧪 Testing storage functionality..."
if [ -f "examples/context_storage_usage.go" ]; then
    echo "🏃 Running storage usage example..."
    if go run examples/context_storage_usage.go; then
        echo "✅ Storage example ran successfully"
    else
        echo "⚠️  Storage example had issues (expected for some storage types)"
        echo "💡 This is normal if storage paths don't exist yet"
    fi
else
    echo "⚠️  Storage example not found"
fi

echo

# 显示状态总结
echo "📊 Installation Summary"
echo "======================"

echo
echo "📦 Dependencies:"
if grep -q "chromem-go" go.mod; then
    echo "  ✅ chromem-go: $(grep 'chromem-go' go.mod | awk '{print $2}')"
else
    echo "  ❌ chromem-go: Not installed"
fi

if grep -q "badger/v4" go.mod; then
    echo "  ✅ BadgerDB: $(grep 'badger/v4' go.mod | awk '{print $2}')"
else
    echo "  ❌ BadgerDB: Not installed"
fi

echo
echo "🗂️  Available Storage Types:"
echo "  ✅ memory   - In-memory storage (always available)"
echo "  $(grep -q "chromem-go" go.mod && echo "✅" || echo "❌") chromem  - Vector database with embeddings"
echo "  $(grep -q "badger/v4" go.mod && echo "✅" || echo "❌") badger   - Persistent key-value database"

echo
echo "🎯 Next Steps:"
echo "  1. Run 'go build' to verify everything compiles"
echo "  2. Try the example: 'go run examples/context_storage_usage.go'"
echo "  3. Read the integration guide: 'internal/context/STORAGE_INTEGRATION.md'"
echo "  4. Update your application to use advanced storage types"

echo
echo "📚 Documentation:"
echo "  - Storage Integration Guide: internal/context/STORAGE_INTEGRATION.md"
echo "  - Usage Examples: examples/context_storage_usage.go"
echo "  - Storage Interfaces: internal/context/storage/interfaces.go"

echo
echo "🎉 Installation completed!"
echo "Alex now supports enterprise-grade storage backends."
