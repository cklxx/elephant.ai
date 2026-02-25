# Contributing to elephant.ai

Thanks for investing in elephant.ai. This repo is opinionated about quality and long-term maintainability; please keep changes focused and reviewable.

## Getting started

```bash
make build
make server-build
```

## Development workflow

- Prefer small, composable changes with clear commit messages.
- Update tests or add new ones when touching logic.
- Keep config changes centralized in `internal/config` and docs.

## Tests

```bash
make test
make test-domain
make test-app
make server-test
```

## Code quality

```bash
make fmt
make vet
```

## Reporting issues

Use GitHub issues with clear reproduction steps, expected vs. actual behavior,
and logs where relevant. For security concerns, see `SECURITY.md`.
