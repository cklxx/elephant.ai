# Release Guide
> Last updated: 2025-11-18


This document explains how to use the newly added automated build and release system.

## 🚀 First Release

### 1. Push Tag to Trigger Automatic Release

```bash
# Create and push tag
git tag v1.0.0
git push origin v1.0.0
```

### 2. Manual Release Trigger (Optional)

You can also manually trigger the Release workflow on the GitHub repository's Actions page:

1. Visit the GitHub repository
2. Click the "Actions" tab
3. Select the "Release" workflow
4. Click "Run workflow"
5. Enter version number (e.g., `v1.0.0`)
6. Click "Run workflow"

## 📦 Release Contents

Each release automatically generates the following files:

- `alex-linux-amd64` - Linux x64 binary
- `alex-linux-arm64` - Linux ARM64 binary  
- `alex-darwin-amd64` - macOS Intel binary
- `alex-darwin-arm64` - macOS Apple Silicon binary
- `alex-windows-amd64.exe` - Windows x64 binary
- `checksums.txt` - SHA256 checksum file

## 🛠️ Installation Methods

After release, users can install through the following methods:

### NPM Install (Recommended)

```bash
npm install -g alex-code
```

### Platform-specific NPM packages

```bash
npm install -g alex-code-linux-amd64     # Linux x64
npm install -g alex-code-linux-arm64     # Linux ARM64  
npm install -g alex-code-darwin-amd64    # macOS Intel
npm install -g alex-code-darwin-arm64    # macOS Apple Silicon
npm install -g alex-code-windows-amd64   # Windows x64
```

### Quick Install (Alternative)

**Linux/macOS:**
```bash
curl -sSfL https://raw.githubusercontent.com/cklxx/Alex-Code/main/scripts/install.sh | sh
```

**Windows:**
```powershell
iwr -useb https://raw.githubusercontent.com/cklxx/Alex-Code/main/scripts/install.ps1 | iex
```

### Manual Download

Users can also directly download the corresponding platform binary from the Releases page.

## ⚙️ Configuration Instructions

### Modify GitHub Repository Path

Before releasing, please ensure you modify the repository path in the following files:

1. **Repository path in installation scripts:**
   - `scripts/install.sh` line 9: `GITHUB_REPO="cklxx/Alex-Code"`
   - `scripts/install.ps1` line 6: `[string]$Repository = "cklxx/Alex-Code"`

2. **Links in documentation:**
   - All GitHub links in `docs/installation.md`

### Version Number Format

Recommend using semantic versioning format:
- `v1.0.0` - Major version
- `v1.1.0` - Minor version  
- `v1.1.1` - Patch version

## 🔍 Verify Release

After release completion, you can verify through the following methods:

1. **Check Releases page:**
   - Confirm all platform binaries have been generated
   - Confirm checksums.txt file exists

2. **Test installation scripts:**
   ```bash
   # Test Linux/macOS installation script
   ./scripts/install.sh --version v1.0.0 --repo your-org/your-repo
   
   # Test Windows installation script
   .\scripts\install.ps1 -Version v1.0.0 -Repository "your-org/your-repo"
   ```

3. **Verify binary files:**
   ```bash
   # Download and test binary
   alex --version
   alex --help
   ```

## 📋 Release Checklist

Before releasing, please confirm:

- [ ] Code has been committed and pushed to main branch
- [ ] Version number updated in code (if needed)
- [ ] Repository path correctly configured in installation scripts
- [ ] Links in documentation updated to correct repository path
- [ ] Main functionality tested and working properly
- [ ] Release notes prepared (GitHub will auto-generate)

## 🐛 Troubleshooting

### Build Failures

If GitHub Actions build fails:

1. Check error logs on Actions page
2. Confirm Go version compatibility
3. Check if dependencies are correct
4. Verify LDFLAGS are correctly set

### Release Failures  

If release process fails:

1. Confirm GITHUB_TOKEN permissions are correct
2. Check if tag format is correct
3. Confirm no duplicate tags exist

### Installation Script Issues

If users report installation problems:

1. Check if binaries are correctly generated
2. Verify download links are valid
3. Confirm file permissions are set correctly

## 📞 Support

If you encounter issues, you can:

1. Check GitHub Actions logs
2. Look for similar issues on the Issues page
3. Create a new Issue for help
