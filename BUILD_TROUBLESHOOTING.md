# Build Troubleshooting Guide

## ✅ Build Works!

```bash
make build
# ✓ Build complete: ./alex
```

If you see this, everything is fine.

---

## ❌ If Build Fails: "go executable not found in PATH"

### Quick Fix (Try First)

```bash
# Option 1: Clean and rebuild
make clean
make build

# Option 2: Use go directly
go build -o alex ./cmd/alex/

# Option 3: Force PATH update
export PATH="/opt/homebrew/bin:$PATH"
make build
```

### Root Causes & Solutions

#### 1. **Go Not Installed**
Check if Go is installed:
```bash
which go
go version
```

If not installed:
```bash
# macOS with Homebrew
brew install go

# Or visit: https://golang.org/dl
```

#### 2. **Go Not in PATH**
Check your PATH:
```bash
echo $PATH | grep -o "/opt/homebrew/bin"
```

Add Go to PATH:
```bash
# Add to ~/.zshrc or ~/.bash_profile
export PATH="/opt/homebrew/bin:$PATH"

# Then reload
source ~/.zshrc  # or source ~/.bash_profile
```

#### 3. **Makefile PATH Issue**
The script `scripts/go-with-toolchain.sh` can't find Go when called from Make.

**Solution**:
```bash
# Method A: Use go directly (simplest)
export GO=/opt/homebrew/bin/go
make build

# Method B: Fix the script
chmod +x scripts/go-with-toolchain.sh
bash scripts/go-with-toolchain.sh build -o alex ./cmd/alex/

# Method C: Use full path
GO=/opt/homebrew/bin/go make build
```

#### 4. **Shell Issue**
The Makefile uses bash, but your shell might be zsh.

**Solution**:
```bash
# Set SHELL explicitly
SHELL=/bin/bash make build
```

### Complete Diagnostic

Run this to diagnose all build issues:

```bash
#!/bin/bash
echo "🔍 Build Diagnostic"
echo "=================="
echo ""

# Check Go
echo "1. Go Installation:"
which go || echo "  ❌ Go not found"
go version || echo "  ❌ Go version check failed"
echo ""

# Check PATH
echo "2. Go in PATH:"
echo $PATH | grep -o "/opt/homebrew/bin" && echo "  ✅ Found" || echo "  ❌ Not in PATH"
echo ""

# Check Make
echo "3. Make Installation:"
which make || echo "  ❌ Make not found"
make --version | head -1 || echo "  ❌ Make version check failed"
echo ""

# Check Makefile
echo "4. Makefile:"
test -f Makefile && echo "  ✅ Makefile exists" || echo "  ❌ Makefile not found"
echo ""

# Check build script
echo "5. Build Script:"
test -f scripts/go-with-toolchain.sh && echo "  ✅ Script exists" || echo "  ❌ Script not found"
test -x scripts/go-with-toolchain.sh && echo "  ✅ Script is executable" || echo "  ❌ Script not executable"
echo ""

# Try build
echo "6. Attempting build..."
make build && echo "  ✅ Build successful" || echo "  ❌ Build failed"
```

Save to a file and run:
```bash
chmod +x build-diag.sh
./build-diag.sh
```

---

## 🔧 Manual Build (If Make Fails)

```bash
# Direct go build (bypasses Makefile entirely)
go build -o alex ./cmd/alex/

# This should always work if Go is installed correctly
```

---

## ⚡ Build Commands Reference

```bash
# Clean build
make clean
make build

# Format + Lint + Build
make dev

# Build specific targets
make build              # Build binary only
make test              # Run tests
make fmt               # Format code
make vet               # Run vet
make clean             # Remove binary

# Troubleshooting
make -n build          # Show what make would do (dry run)
```

---

## 🛠️ Environment Variables

Set these if build is failing:

```bash
# Force Go executable location
export GO=/opt/homebrew/bin/go

# Force shell
export SHELL=/bin/bash

# Force PATH
export PATH="/opt/homebrew/bin:$PATH"

# Then try
make build
```

---

## 📊 Common Build Errors & Fixes

| Error | Cause | Fix |
|-------|-------|-----|
| `go executable not found` | Go not in PATH | `export PATH="/opt/homebrew/bin:$PATH"` |
| `permission denied` | Script not executable | `chmod +x scripts/go-with-toolchain.sh` |
| `command not found: make` | Make not installed | `brew install make` (macOS) |
| `module not found` | Missing dependencies | `go mod download` |
| `version ... does not exist` | Wrong Go version | `go mod tidy` |

---

## ✅ Verification Checklist

After fixing build issues, verify:

- [ ] `which go` shows `/opt/homebrew/bin/go`
- [ ] `go version` shows Go 1.21+
- [ ] `make build` completes without errors
- [ ] `./alex --version` works
- [ ] `./alex --no-tui version` works
- [ ] `scripts/diagnose-startup.sh` passes

---

## 🚀 If Everything Else Fails

Use Go directly, bypassing Make:

```bash
# Build
go build -o alex ./cmd/alex/

# Run
./alex --no-tui

# Test
go test ./...
```

This should always work if Go is installed, regardless of Makefile issues.

---

## 📝 Reporting Build Issues

If the above doesn't fix it, provide:

```bash
# Run these commands and share output
which go
go version
echo $PATH | grep homebrew
go env GOPATH
make -n build
```

Then file an issue with the output.
