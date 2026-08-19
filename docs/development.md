# Development

- [Building](#building)
- [Git hooks](#git-hooks)
- [Running tests](#running-tests)
- [Coverage](#coverage)
- [Formatting and linting](#formatting-and-linting)
- [Project structure](#project-structure)
- [Architecture notes](#architecture-notes)
- [Guidelines](#guidelines)

## Building

```bash
git clone https://github.com/pradyb/sgh-cli.git
cd sgh-cli
go mod download
go build -o sgh .
```

Release binaries are built with version metadata injected at link time:

```bash
go build -ldflags="-s -w \
  -X github.com/pradyb/sgh-cli/cmd/version.Version=v1.2.3 \
  -X github.com/pradyb/sgh-cli/cmd/version.CommitSHA=$(git rev-parse --short HEAD) \
  -X github.com/pradyb/sgh-cli/cmd/version.BuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o sgh .
```

Without those flags `sgh version` reports `dev`.

## Git hooks

Enable once per clone:

```bash
git config core.hooksPath .githooks
```

The pre-commit hook blocks three things, all cheap to check and expensive to
discover later: files over 1 MB, strings that look like real GitHub tokens, and
Go files that are not gofmt-formatted. It inspects only staged content and never
runs tests, so it costs well under a second. `git commit --no-verify` bypasses
it; CI enforces the same rules either way.

## Running tests

```bash
go test ./...              # everything
go test ./... -v           # verbose
go test ./... -race        # race detector
go test ./internal/config  # one package
```

CI runs `go test -v -race -short ./...`. Tests that need more than a moment should respect `testing.Short()` so the release path stays fast.

## Coverage

```bash
# Percentage for one package
go test ./internal/config -cover

# Profile, then read it two ways
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

Open `coverage.html` in a browser to see exactly which branches are untested.

## Formatting and linting

`gofmt` is the standard CI enforces — the build fails on any drift:

```bash
gofmt -l .    # list unformatted files
gofmt -w .    # fix them
go vet ./...
```

> **golangci-lint is currently dormant.** It does not yet ship a build
> supporting the Go version this project targets, so the CI lint job is
> commented out and `.golangci.yml` is unused. Re-enable both once upstream
> catches up.

## Project structure

```
sgh-cli/
├── main.go                 # entrypoint, token validation, error formatting
├── cmd/                    # one package per command — Cobra wiring only
│   ├── root.go             # root command, global flags, group registration
│   ├── shortcuts.go        # single-word shortcut expansion (brl, prl, ...)
│   ├── audit/  branch/  clone/  commit/  config/  health/  issue/
│   ├── org/  postrelease/  pr/  protectedbranch/  repo/  security/
│   └── tag/  team/  tui/  version/  whoami/  workflow/
├── pkg/                    # business logic — importable, no Cobra dependency
│   ├── apperrors/          # typed application errors
│   ├── context/            # global application context, token resolution
│   ├── logger/             # structured logging (zerolog)
│   ├── ui/                 # table rendering, colors, progress bars
│   ├── validation/         # token and input validation
│   ├── utils/              # shared helpers
│   └── audit/ branch/ clone/ commit/ config/ issue/ org/ postrelease/
│       pr/ (+ pr/prompt/)  protectedbranch/ repo/ security/ tag/
│       team/ whoami/ workflow/
├── internal/               # infrastructure — not importable by other modules
│   ├── async/              # job queue and worker pool
│   ├── circuitbreaker/     # circuit breaker for failing endpoints
│   ├── client/             # HTTP client with retry + rate limit interceptor
│   ├── config/             # config file load, save, validation
│   ├── model/              # data models and GraphQL queries
│   ├── processor/          # bulk repository operation processor
│   ├── ratelimit/          # GitHub API rate limit tracking
│   ├── retry/              # exponential backoff
│   ├── service/            # GitHub REST and GraphQL services
│   └── testutils/          # shared test helpers
└── utils/                  # legacy helpers
```

The layering rule: `cmd/` parses flags and prints, `pkg/` holds logic that could be called from anywhere, `internal/` holds infrastructure nobody outside this module should depend on. A command should be thin enough to read in one screen.

## Architecture notes

**Bulk operations** flow through `internal/processor`, which fans work across a worker pool (`internal/async`) and collects per-repository results so one failure never aborts the batch.

**Resilience** is layered in the HTTP client: `internal/retry` handles transient failures with exponential backoff, `internal/ratelimit` reads GitHub's quota headers and paces requests, and `internal/circuitbreaker` stops hammering an endpoint that is consistently failing.

**Two APIs.** Most operations use REST via `internal/service/github_api_service.go`. Queries that would otherwise need many round-trips use GraphQL via `github_graphql_service.go`.

## Guidelines

- Add tests for new features and bug fixes. The coverage targets are **85% for
  `pkg/**` (except `pkg/pr/prompt`) and `internal/**`**, with no floor on
  `cmd/**` — see [CONTRIBUTING](../CONTRIBUTING.md#coverage-expectations) for
  the reasoning. These targets are not met yet and apply to new work
- Run `go test ./...` and `gofmt -l .` before opening a pull request
- Keep `cmd/` packages thin — logic belongs in `pkg/`
- Anything destructive should support `--dry-run`
- Follow standard Go conventions; the linter enforces most of them

See [CONTRIBUTING.md](../CONTRIBUTING.md) for the pull request process.
