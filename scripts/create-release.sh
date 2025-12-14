#!/bin/bash

# Create GitHub Release Script
# This script helps create a GitHub release with all the built binaries

set -e

VERSION="v1.0.0"
REPO="cklxx/Alex-Code"

echo "Creating release $VERSION for $REPO"
echo ""
echo "Built binaries available in build/:"
ls -la build/

echo ""
echo "To create the release manually:"
echo "1. Go to https://github.com/$REPO/releases/new"
echo "2. Tag version: $VERSION"
echo "3. Release title: Alex CLI $VERSION - Full-Featured Software Engineering Assistant"
echo "4. Upload the following files from build/:"
echo "   - alex-linux-amd64"
echo "   - alex-linux-arm64" 
echo "   - alex-darwin-amd64"
echo "   - alex-darwin-arm64"
echo "   - alex-windows-amd64.exe"

echo ""
echo "Release notes template:"
cat << 'EOF'
# Alex CLI v1.0.0 🚀

A production-ready AI software engineering assistant built in Go with ReAct agent architecture.

## ✨ Features

- **ReAct Agent Architecture**: Think-Act-Observe cycle with streaming support
- **MCP Protocol Integration**: Model Context Protocol with JSON-RPC 2.0
- **Memory Management**: Dual-layer with vector storage for context preservation
- **12+ Built-in Tools**: File operations, shell commands, search, web search, and more
- **Session Management**: Persistent storage with automatic compression
- **SWE-Bench Evaluation**: Built-in evaluation system with parallel processing
- **Multi-Model LLM Support**: OpenRouter, DeepSeek, and more
- **Modern Terminal UI**: Interactive interface with streaming responses

## 📦 Installation

### Quick Install (Linux/macOS)
```bash
curl -sSfL https://raw.githubusercontent.com/cklxx/Alex-Code/main/scripts/install.sh | sh
```

### Windows (PowerShell)
```powershell
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/cklxx/Alex-Code/main/scripts/install.ps1" -OutFile "install.ps1"; .\install.ps1
```

### Manual Download
Download the appropriate binary for your platform from the assets below.

## 🚀 Quick Start

```bash
# Interactive mode
alex

# Single command
alex "Analyze this codebase and suggest improvements"

# Resume a session
alex -r session_id -i
```

## 📖 Documentation

Visit our documentation at [docs/](./docs/) for detailed guides and API reference.

## 🏗️ Architecture

- **Core**: ReAct agent with streaming and memory
- **Tools**: Extensible tool system with MCP integration  
- **Memory**: Vector-based context management
- **Security**: Risk assessment and approvals
- **Performance**: Sub-30ms execution, <100MB memory

Built with ❤️ in Go for maximum performance and reliability.
EOF
