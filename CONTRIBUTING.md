# Contributing

Thanks for contributing to `ttscli`.

## Prerequisites

- Go (see `go.mod` for the project version baseline)
- A Google Cloud Text-to-Speech API key for manual CLI testing

## Local Setup

```bash
make tools
export PATH="$(go env GOPATH)/bin:$PATH"
```

## Development Workflow

1. Create a feature branch from `main`.
2. Make focused changes with tests.
3. Run quality checks before opening a PR:

```bash
make check
```

4. Update docs when behavior or flags change.

## Commit Style

Use conventional-style prefixes where possible:

- `feat:` for user-facing features
- `fix:` for bug fixes
- `refactor:` for structure/maintainability
- `test:` for test-only changes
- `docs:` for documentation updates
- `chore:` for tooling/automation changes

## Release Process

- Tags matching `v*.*.*` trigger the release workflow.
- Releases are built with GoReleaser from `.goreleaser.yml`.
- Built binaries include version metadata visible via `ttscli --version`.
