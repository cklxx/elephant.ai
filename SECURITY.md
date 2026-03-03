# Security Policy

## Supported Versions

We actively maintain security fixes for the latest release on `main`.

| Version | Supported |
|---------|-----------|
| `main` (latest) | ✅ |
| Older commits | ❌ |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

To report a security issue, use one of the following:

1. **GitHub Security Advisories** *(preferred)*: [Report a vulnerability](https://github.com/cklxx/elephant.ai/security/advisories/new)
2. **Email**: Contact the maintainer directly via GitHub profile.

Please include as much of the following as possible:

- Type of issue (e.g. command injection, credential leak, prompt injection, SSRF)
- Full paths of affected source files
- Location of the vulnerable code (tag/branch/commit or direct URL)
- Any special configuration required to reproduce
- Step-by-step instructions to reproduce
- Proof-of-concept or exploit code (if possible)
- Impact assessment — what an attacker could achieve

## Response Timeline

| Stage | Target |
|-------|--------|
| Initial acknowledgement | Within 3 business days |
| Severity assessment | Within 7 business days |
| Fix or mitigation | Depends on severity (critical: ≤14 days) |
| Public disclosure | After fix is released |

## Scope

### In scope

- Remote code execution or command injection via any input surface (Lark messages, CLI args, API endpoints, config files)
- Prompt injection attacks that bypass approval gates or leak system context
- Credential or secret leakage (LLM API keys, Lark tokens, OAuth tokens)
- Authentication or authorization bypass in the API server
- SSRF through the browser tool or MCP clients
- Unintended data persistence or cross-session memory leakage
- Denial-of-service via resource exhaustion in the sandbox

### Out of scope

- Issues in third-party LLM provider APIs (report to the respective provider)
- Social engineering attacks
- Physical attacks
- Issues requiring the attacker to already have shell access to the host

## Security Design Notes

elephant.ai executes code and browses the web on behalf of users. Key security boundaries:

- **Sandboxed code execution** — code runs in an isolated container, not the host
- **Approval gates** — destructive or irreversible operations require explicit user sign-off
- **Credential isolation** — LLM keys and Lark tokens are read from environment/config, never logged or stored in memory
- **Prompt injection awareness** — tool outputs and external content are treated as untrusted

## Attribution

We will publicly thank reporters in release notes (unless they prefer to remain anonymous).
