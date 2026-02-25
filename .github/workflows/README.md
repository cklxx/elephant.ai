# 🚀 GitHub Actions Workflows

This directory contains automated workflows for building, testing, and publishing Alex-Code.

## 📋 Available Workflows

### 1. 📦 NPM Package Publishing (`npm-publish.yml`)

**Triggers:**
- Version tags: `v*`, `npm-v*` (e.g., `v0.0.5`, `npm-v0.0.5`)
- Manual dispatch with version input

**Features:**
- Builds Go binaries for all platforms
- Updates package versions automatically  
- Publishes to NPM registry
- Includes installation testing
- Supports dry-run mode

**Usage:**
```bash
# Trigger via tag
git tag npm-v0.0.5
git push origin npm-v0.0.5

# Or use GitHub UI: Actions → NPM Package Publishing → Run workflow
```

### 2. ⚡ Quick NPM Publish (`quick-npm-publish.yml`)

**Triggers:**
- Manual dispatch only

**Features:**
- Fast publishing without extensive testing
- Option to skip installation tests
- Uses existing `make release-npm` command

**Usage:**
- GitHub UI: Actions → Quick NPM Publish → Run workflow
- Input version and optionally skip tests

### 3. 🎉 Release (`release.yml`)

**Triggers:**
- Version tags: `v*`
- Manual dispatch

**Features:**
- Creates GitHub releases with binaries
- Generates platform archives and checksums
- Optional NPM publishing
- Rich release notes

**Usage:**
```bash
# Trigger via tag  
git tag v0.0.5
git push origin v0.0.5

# Or use GitHub UI: Actions → Release → Run workflow
```

## 🔧 Setup Requirements

### NPM Token Setup

1. **Get NPM Token:**
   ```bash
   npm login
   npm token create --type=automation
   ```

2. **Add to GitHub Secrets:**
   - Go to: Repository Settings → Secrets and variables → Actions
   - Add secret: `NPM_TOKEN` = `your_npm_token`

### Required Secrets

| Secret | Description | Required For |
|--------|-------------|--------------|
| `NPM_TOKEN` | NPM automation token | NPM publishing |
| `GITHUB_TOKEN` | Auto-generated | GitHub releases |

## 🎯 Workflow Strategies

### For Regular Releases
```bash
# 1. Update version and test locally
make update-version VERSION=0.0.5
make test

# 2. Commit and tag
git add -A
git commit -m "Release v0.0.5"
git tag v0.0.5
git push origin main v0.0.5

# 3. GitHub Actions will:
#    - Build binaries
#    - Create GitHub release  
#    - Publish NPM packages
#    - Test installation
```

### For NPM-Only Releases
```bash
# Quick NPM publish without GitHub release
git tag npm-v0.0.5
git push origin npm-v0.0.5
```

### For Emergency Releases
- Use "Quick NPM Publish" workflow
- Manual trigger with version input
- Skip tests if needed

## 📊 Workflow Outputs

### NPM Publishing
- **Packages:** `alex-code`, `alex-code-{platform}`
- **Registry:** https://www.npmjs.com/package/alex-code
- **Installation:** `npm install -g alex-code@{version}`

### GitHub Releases  
- **Archives:** Platform-specific tar.gz/zip files
- **Checksums:** SHA256 verification file
- **Assets:** Ready-to-use binaries

### Testing
- **Installation:** Automated NPM install test
- **Commands:** `alex version`, `alex --help`
- **Verification:** Command availability check

## 🐛 Troubleshooting

### NPM Publishing Fails
- Check `NPM_TOKEN` secret is valid
- Verify version doesn't already exist
- Check package.json syntax

### Build Failures
- Ensure Go code compiles locally
- Check dependency compatibility
- Verify Makefile targets work

### GitHub Release Issues
- Verify tag format (`v*`)
- Check repository permissions
- Ensure binary builds succeed

## 🔄 Workflow Dependencies

```
npm-publish.yml:
  ├── build-binaries → publish-npm → test-installation
  
quick-npm-publish.yml:
  └── quick-publish (single job)
  
release.yml:
  └── release → publish-npm (optional)
```

## 💡 Best Practices

1. **Version Naming:**
   - GitHub releases: `v0.0.5`  
   - NPM-only: `npm-v0.0.5`
   - Consistent semver format

2. **Testing:**
   - Always test locally first
   - Use dry-run for validation
   - Monitor installation tests

3. **Rollback:**
   - Delete problematic tags
   - Use `npm unpublish` if needed
   - Update documentation

4. **Security:**
   - Rotate NPM tokens regularly
   - Use automation tokens only
   - Monitor secret usage