# Contributing to CertAuto

Thanks for your interest in contributing! This document explains how to file issues, run tests, format code, and prepare a PR.

## Reporting issues

- Search existing issues before opening a new one.
- Provide a clear title, reproduction steps, expected vs actual behaviour, and logs if applicable.

## Development workflow

1. Fork the repository and create a branch named `feat/<short-desc>` or `fix/<short-desc>`.
2. Implement changes with small, focused commits.
3. Run unit tests and linters locally.

Recommended commands:

```bash
# Run unit tests
go test ./... -v

# Run linters (install golangci-lint)
golangci-lint run

# Format code
gofmt -s -w .

# Regenerate code if API types changed
make generate
```

## Commit messages

- Use conventional, concise commit messages. Start with a short prefix: `feat:`, `fix:`, `docs:`, `chore:`.
- Include a one-line summary and an optional longer description.

## Pull request checklist

- Update or add unit tests for new behavior.
- Run `golangci-lint` and `gofmt`.
- Ensure CI passes.
- Add a short description of the change and testing steps in the PR body.

## Coding guidelines

- Follow Go idioms and keep changes minimal and focused.
- Avoid committing secrets or private keys — use Kubernetes Secrets instead.
- Update `README.md` and `docs/` for user-facing changes.

Thank you — maintainers will review your PR and request changes if needed.
