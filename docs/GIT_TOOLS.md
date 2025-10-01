# Git Tools Documentation

ALEX includes comprehensive Git integration tools that enable automatic commits with AI-generated messages and GitHub PR creation.

## Available Tools

### 1. git_commit - AI-Powered Commit Tool

Automatically creates git commits with AI-generated conventional commit messages.

**Features:**
- Detects modified files via `git status`
- Generates commit messages following Conventional Commits format
- Interactive approval mode (default)
- Automatic commit mode with `--auto` flag
- Custom message support with `--message` parameter
- Handles both staged and unstaged changes
- Adds ALEX attribution footer

**Usage Examples:**

```bash
# Get proposed commit message for review (interactive mode)
alex commit

# Auto-commit with AI-generated message
alex commit --auto

# Use custom commit message
alex commit --message "feat: add new feature" --auto

# Let ALEX generate the message but review first
alex commit
# Review the proposed message, then run with --auto to commit
```

**Commit Message Format:**

Generated messages follow the Conventional Commits specification:

```
<type>: <description>

<optional body>

🤖 Generated with ALEX
Co-Authored-By: ALEX <noreply@alex.com>
```

**Supported Types:**
- `feat:` - New feature
- `fix:` - Bug fix
- `refactor:` - Code refactoring
- `docs:` - Documentation changes
- `test:` - Test additions/changes
- `chore:` - Build/tooling changes
- `style:` - Code style changes

**Tool Parameters:**

```json
{
  "message": "string (optional) - Custom commit message",
  "auto": "boolean (optional) - Auto-commit without approval (default: false)"
}
```

### 2. git_pr - GitHub Pull Request Creator

Creates GitHub pull requests with AI-generated titles and descriptions.

**Features:**
- Auto-detects default branch (main/master)
- Generates comprehensive PR descriptions from commits and diffs
- Structured PR format with Summary, Changes, and Test Plan sections
- Automatically pushes branch if not already on remote
- Uses GitHub CLI (`gh`) for PR creation
- Custom title support
- ALEX attribution footer

**Prerequisites:**
- GitHub CLI (`gh`) must be installed
- Must be authenticated with GitHub: `gh auth login`
- Repository must have a remote configured

**Usage Examples:**

```bash
# Create PR with AI-generated title and description
alex pr

# Create PR with custom title
alex pr --title "Add authentication feature"

# Create PR targeting a specific base branch
alex pr --base develop
```

**PR Description Structure:**

```markdown
## Summary
- Key change 1
- Key change 2
- Key change 3

## Changes
Detailed breakdown of changes by component/file:
- Component A: Description of changes
- Component B: Description of changes

## Test Plan
- How to verify change 1
- How to verify change 2
- Integration test steps

---
🤖 Generated with ALEX
```

**Tool Parameters:**

```json
{
  "title": "string (optional) - Custom PR title",
  "base": "string (optional) - Base branch (default: auto-detect main/master)"
}
```

### 3. git_history - Commit History Search

Search git commit history by various criteria.

**Features:**
- Search commit messages (text matching)
- Search code changes (pickaxe search)
- File history tracking
- Author-based search
- Date-based search with flexible formats

**Search Types:**

#### Message Search (default)
Search commit messages for specific text:

```bash
alex git-search "authentication"
alex git-search "fix bug"
```

```json
{
  "query": "authentication",
  "type": "message",
  "limit": 20
}
```

#### Code Search
Find commits where specific code was added or removed:

```bash
alex git-search "function authenticate" --type code
```

```json
{
  "query": "function authenticate",
  "type": "code",
  "limit": 20
}
```

#### File History
Get commit history for a specific file:

```bash
alex git-search --type file --file src/auth.go
```

```json
{
  "type": "file",
  "file": "src/auth.go",
  "limit": 20
}
```

#### Author Search
Find commits by author:

```bash
alex git-search "john@example.com" --type author
```

```json
{
  "query": "john@example.com",
  "type": "author",
  "limit": 20
}
```

#### Date Search
Search commits by date or date range:

```bash
# Relative dates
alex git-search "last week" --type date
alex git-search "3 days ago" --type date

# Specific date
alex git-search "2024-01-01" --type date

# Date range
alex git-search "2024-01-01..2024-12-31" --type date
```

```json
{
  "query": "last week",
  "type": "date",
  "limit": 20
}
```

**Tool Parameters:**

```json
{
  "query": "string (optional) - Search query",
  "type": "string (optional) - Search type: message, code, file, author, date",
  "file": "string (optional) - File path (required when type=file)",
  "limit": "number (optional) - Max results (default: 20)"
}
```

## Integration with ALEX Agent

All Git tools are automatically available to the ALEX agent and can be used during task execution.

**Example Agent Interactions:**

```
User: "Commit my changes with a good message"
ALEX: [Uses git_commit tool to analyze changes and generate commit message]

User: "Create a PR for this feature"
ALEX: [Uses git_pr tool to analyze commits and create comprehensive PR]

User: "When did we last change the auth system?"
ALEX: [Uses git_history tool to search commit history]
```

## Installation Requirements

### git_commit and git_history
- Git must be installed and in PATH
- Repository must be initialized (`git init`)

### git_pr (additional requirements)
- GitHub CLI (`gh`) must be installed
- Authenticate with GitHub: `gh auth login`
- Repository must have GitHub remote configured
- Proper GitHub permissions (write access)

### Installing GitHub CLI

**macOS:**
```bash
brew install gh
```

**Linux:**
```bash
# Debian/Ubuntu
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
sudo apt update
sudo apt install gh

# Fedora/CentOS
sudo dnf install gh
```

**Windows:**
```bash
winget install --id GitHub.cli
```

## Architecture

### Tool Implementation

All Git tools are implemented in `/internal/tools/builtin/`:

- `git_commit.go` - Commit tool with LLM integration
- `git_pr.go` - PR tool with LLM integration
- `git_history.go` - History search tool (no LLM needed)

### LLM Integration

`git_commit` and `git_pr` tools require an LLM client for message/description generation. The client is injected during tool registration in the container.

**Registration Flow:**
1. Container creates LLM client from factory
2. Calls `toolRegistry.RegisterGitTools(llmClient)`
3. Git tools are registered with LLM client
4. Tools become available to the agent

**Code Reference:**
```go
// cmd/alex/container.go
llmClient, err := llmFactory.GetClient(config.LLMProvider, config.LLMModel, llm.Config{
    APIKey:  config.APIKey,
    BaseURL: config.BaseURL,
})
if err == nil {
    toolRegistry.RegisterGitTools(llmClient)
}
```

### Testing

Comprehensive test coverage includes:

1. **Unit Tests** (`*_test.go`)
   - Tool definition and metadata
   - Parameter validation
   - Message/description generation
   - Footer formatting
   - Error handling

2. **Integration Tests** (`git_integration_test.go`)
   - Full workflow testing with real git repository
   - Commit creation and verification
   - History search across different types
   - Custom message handling

**Running Tests:**

```bash
# All Git tests
go test ./internal/tools/builtin -run TestGit -v

# Specific tool tests
go test ./internal/tools/builtin -run TestGitCommit -v
go test ./internal/tools/builtin -run TestGitPR -v
go test ./internal/tools/builtin -run TestGitHistory -v

# Integration tests (requires git)
go test ./internal/tools/builtin -run TestGitIntegration -v
```

## Best Practices

### Commit Messages

1. **Let AI Generate First**: The AI analyzes your diff and generates contextual messages
2. **Review Before Committing**: Use interactive mode (default) to review messages
3. **Use Custom Messages for Simple Changes**: Override with `--message` for trivial commits
4. **Follow Conventional Commits**: AI is trained to follow this format

### Pull Requests

1. **Ensure Clean History**: Squash/rebase commits before creating PR
2. **Review Generated Description**: AI-generated descriptions are comprehensive but review for accuracy
3. **Add Custom Context**: Use custom title for important context the AI might miss
4. **Verify Test Plan**: Ensure the generated test plan matches your testing approach

### History Searches

1. **Use Specific Queries**: More specific queries yield better results
2. **Combine Search Types**: Use message search to find commits, then file search for specific changes
3. **Leverage Date Ranges**: Use date search for temporal analysis
4. **Check File History**: Use file search to understand evolution of specific files

## Troubleshooting

### "not a git repository"
- Ensure you're in a git repository
- Run `git init` if starting a new repository

### "no changes to commit"
- Check `git status` to see if there are changes
- Stage files with `git add` before committing

### "gh CLI not installed"
- Install GitHub CLI (see Installation Requirements)
- Verify with `gh --version`

### "failed to create PR"
- Ensure you're authenticated: `gh auth login`
- Verify remote is configured: `git remote -v`
- Check you have write access to the repository
- Ensure branch is pushed to remote

### "LLM client not configured"
- Verify ALEX_API_KEY environment variable is set
- Check LLM provider configuration
- Ensure network connectivity to LLM API

## Examples

### Example 1: Complete Feature Development Workflow

```bash
# Make changes to files
echo "new feature" > feature.go

# Stage changes
git add feature.go

# Commit with AI-generated message (review mode)
alex commit
# Review the proposed message...

# Commit for real
alex commit --auto

# Create PR
alex pr
# PR created with comprehensive description
```

### Example 2: Finding When Code Changed

```bash
# When was authentication added?
alex git-search "authentication" --type code

# What changes did John make last week?
alex git-search "john@example.com" --type author

# History of auth.go file
alex git-search --type file --file src/auth.go
```

### Example 3: Custom Commit Message

```bash
# Use custom message for simple change
alex commit --message "docs: fix typo in README" --auto
```

## Future Enhancements

Potential future features:

- Branch management (create, switch, delete)
- Merge conflict resolution assistance
- PR review automation
- Commit squashing assistance
- Tag management
- Release note generation
- Changelog generation from commits
- Git hook management

## Contributing

When adding new Git functionality:

1. Follow the existing tool pattern (see `git_history.go` for simple tools)
2. Add comprehensive unit tests
3. Add integration tests if interacting with git
4. Update this documentation
5. Test with real repositories
6. Consider error cases (not in repo, no changes, etc.)

## License

Same as ALEX project license.
