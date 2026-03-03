# Contributing to elephant.ai

Thanks for your interest in contributing. This project values quality over velocity — small, well-reasoned changes beat large, unfocused ones.

## Table of Contents

- [Development Setup](#development-setup)
- [Architecture Overview](#architecture-overview)
- [Code Standards](#code-standards)
- [PR Workflow](#pr-workflow)
- [Adding a Skill](#adding-a-skill)
- [Reporting Issues](#reporting-issues)

---

## Development Setup

**Prerequisites:** Go 1.24+, Node.js 20+, Docker (for sandbox)

```bash
git clone https://github.com/cklxx/elephant.ai.git
cd elephant.ai

# Build the alex binary
make build

# Copy and edit config
cp examples/config/runtime-config.yaml ~/.alex/config.yaml
export LLM_API_KEY="sk-..."

# Start all services
alex dev up

# Run tests
alex dev test        # Go tests with race detector
alex dev lint        # Go + web lint
npm --prefix web run test   # Frontend unit tests
```

For Lark bot setup, see [`docs/guides/quickstart.md`](docs/guides/quickstart.md).

---

## Architecture Overview

```
cmd/alex              CLI entrypoint
cmd/alex-server       HTTP API server
internal/
  app/                Application layer — coordination, context assembly, DI
  domain/             Domain logic — ReAct loop, events, approval gates
  delivery/           Delivery adapters — Lark, HTTP server
  infra/              Infrastructure — LLM, memory, tools, MCP, observability
web/                  Next.js dashboard (SSE streaming, session management)
skills/               Markdown-driven skill workflows
```

**Key design rules:**
- Cross-layer imports go through port interfaces (`internal/*/ports/`). Direct infra→domain imports are forbidden.
- New LLM features must work across all providers in `internal/infra/llm/`. No provider-specific APIs without an adapter.
- Typed events over log strings for state transitions (`internal/domain/agent/events/`).

---

## Code Standards

**Go:**
- Follow standard Go conventions (`gofmt`, `go vet`)
- Return errors directly; avoid defensive wrapping when the caller contract guarantees safety
- Delete dead code completely — no `// deprecated` markers or `_unused` prefixes
- One struct/interface per responsibility; no god-structs

**TypeScript/React:**
- Strict TypeScript — no `any` unless genuinely unavoidable
- Components in `web/components/`, hooks in `web/hooks/`
- SSE event types mirror the Go backend types in `web/lib/types/`

**Commits:**
```
feat(lark): add approval gate for calendar writes
fix(kernel): guard MarkDispatchRunning failure on nil dispatch
test(llm): add streaming error branch coverage
docs: update CONFIG.md with new scheduler fields
```

Format: `type(scope): description` — types: `feat`, `fix`, `test`, `docs`, `chore`, `refactor`

---

## PR Workflow

1. **Fork** the repo and create a branch: `git checkout -b feat/your-feature`
2. **Write tests** for any logic changes (TDD preferred)
3. **Lint and test:** `alex dev lint && alex dev test`
4. **Open a PR** against `main` with a clear description of what and why
5. Address review feedback; maintainers aim to respond within a few days

**Branch naming:** `feat/`, `fix/`, `test/`, `docs/`, `refactor/` prefixes.

**PR checklist:**
- [ ] Tests pass (`alex dev test`)
- [ ] Lint clean (`alex dev lint`)
- [ ] No dead code or commented-out blocks left behind
- [ ] Config changes documented in `docs/reference/CONFIG.md`
- [ ] Breaking changes noted in PR description

---

## Adding a Skill

Skills are markdown files in `skills/` that the agent loads and executes. To add one:

```bash
mkdir skills/my-skill
touch skills/my-skill/SKILL.md
```

**`SKILL.md` structure:**
```markdown
# Skill Name

## Trigger
Describe when this skill should activate (keywords, intent patterns).

## Steps
1. Step one — what the agent does
2. Step two — tool calls and logic
3. Step three — output format

## Output
Describe what the skill produces (artifact, message, structured data).
```

Skills are plain markdown — no code required. The ReAct loop interprets them at runtime. See existing skills in `skills/` for examples.

---

## Reporting Issues

Use [GitHub Issues](https://github.com/cklxx/elephant.ai/issues) with:

- Clear title summarizing the problem
- Steps to reproduce
- Expected vs. actual behavior
- Relevant logs (`alex dev logs server`)
- Environment: OS, Go version, LLM provider

For security vulnerabilities, see [`SECURITY.md`](SECURITY.md) — do not open a public issue.

**Good first issues** are labeled [`good first issue`](https://github.com/cklxx/elephant.ai/issues?q=label%3A%22good+first+issue%22).
