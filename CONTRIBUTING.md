# Contributing to sgh-cli

Thank you for your interest in contributing to sgh-cli! This document outlines the process and guidelines for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [How to Contribute](#how-to-contribute)
- [Pull Request Process](#pull-request-process)
- [Coding Standards](#coding-standards)
- [Reporting Bugs](#reporting-bugs)
- [Requesting Features](#requesting-features)

## Code of Conduct

By participating in this project, you agree to be respectful and considerate of others. We are committed to providing a welcoming and inclusive environment.

## Getting Started

1. **Fork** the repository on GitHub
2. **Clone** your fork locally:
   ```bash
   git clone https://github.com/your-username/sgh-cli.git
   cd sgh-cli
   ```
3. **Add the upstream remote:**
   ```bash
   git remote add upstream https://github.com/pradyb/sgh-cli.git
   ```

## Development Setup

### Prerequisites

- **Go 1.24.0 or higher**
- **GitHub Personal Access Token** with `repo` and `admin:org` scopes (for integration tests)

### Enable the git hooks

Do this once per clone. It wires up a fast pre-commit check that blocks large
files, accidental credentials, and unformatted Go:

```bash
git config core.hooksPath .githooks
```

The hook only inspects staged content and never runs the test suite, so it adds
well under a second. Bypass it with `git commit --no-verify` if you genuinely
need to — CI enforces the same rules regardless.

### Build

```bash
# Build the binary
go build -o sgh .

# Run tests
go test ./...

# Run tests with race detector
go test -race ./...

# Check formatting — CI fails on any drift
gofmt -l .
```

> **golangci-lint is not currently runnable.** It does not yet ship a build
> supporting the Go version this project targets, so the CI lint job is
> disabled and `.golangci.yml` is dormant. `gofmt` and `go vet` are the
> standards enforced today.

### Environment Variables

```bash
export SGH_TOKEN=your_personal_access_token
export SGH_ORG=your-test-org   # optional, for integration tests
```

## How to Contribute

### Workflow

1. **Sync** with upstream before starting:
   ```bash
   git fetch upstream
   git rebase upstream/develop
   ```
2. **Create a branch** from `develop`:
   ```bash
   git checkout -b feature/your-feature-name
   ```
3. **Make your changes** following the coding standards below
4. **Write or update tests** for your changes
5. **Run the full test suite** to ensure nothing is broken
6. **Commit** with a clear message (see commit message format below)
7. **Push** to your fork and open a Pull Request

### Commit Message Format

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <short summary>

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`

Examples:
```
feat(pr): add bulk merge with auto-delete branch option
fix(branch): handle repos without default branch gracefully
docs: update README with new workflow examples
```

## Pull Request Process

1. Ensure all tests pass and the build succeeds
2. Update the README if your change affects user-facing behaviour
3. Update `CHANGELOG.md` under the `[Unreleased]` section
4. Reference any related issues using `Fixes #123` or `Closes #123`
5. Request a review from a maintainer
6. PRs must target the `develop` branch (not `main`)

## Coding Standards

- Follow standard Go formatting (`gofmt` / `goimports`)
- All exported types and functions must have doc comments
- Keep functions focused and small; prefer composition
- Add license headers to all new `.go` files:
  ```go
  // Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
  // SPDX-License-Identifier: MIT
  ```
- Write table-driven tests where appropriate
- Do not commit binaries or build artifacts

## Reporting Bugs

Please open an issue using the **Bug Report** template and include:

- sgh-cli version (`sgh version`)
- Go version and OS
- Steps to reproduce
- Expected vs actual behaviour
- Any relevant logs (run with `--verbose` flag)

**Security vulnerabilities** must not be reported as public issues. See [SECURITY.md](SECURITY.md) for the responsible disclosure process.

## Requesting Features

Open an issue using the **Feature Request** template. Describe:

- The problem you are trying to solve
- Your proposed solution
- Any alternatives you considered
