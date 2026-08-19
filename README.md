# 🚀 Simple GitHub CLI (sgh-cli)

Bulk GitHub operations across an entire organization. Create a release branch in 200 repositories, review every open PR your team filed this week, or find which repos still have branch protection disabled — with one command instead of a shell loop.

[![CI](https://github.com/pradyb/sgh-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/pradyb/sgh-cli/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.26.1+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

> **Disclaimer:** This project is an independent, community-built tool and is not affiliated with, endorsed by, or sponsored by GitHub, Inc. "GitHub" is a trademark of GitHub, Inc.

## Why this exists

GitHub's own `gh` is excellent and repository-scoped by design. `sgh` is the other axis: every command takes an organization and fans out across its repositories concurrently, with regex include/exclude filtering, a shared worker pool, rate-limit handling, and `--dry-run` on anything destructive.

```bash
sgh branch create --org my-org --new Release-2.0 --ref main --dry-run
sgh pr list --org my-org --state open --sort repo
sgh workflow list --org my-org --failed --branch main
sgh protected-branch list --org my-org --branch main
```

## Features

- **Branches and tags** — create, rename, delete, and filter across every repository
- **Pull requests** — create, list, view, review, update, merge, close, reopen in bulk, plus an interactive selector
- **GitHub Actions** — list, view, rerun, cancel, and dispatch workflow runs, with live monitoring
- **Branch protection** — inspect and update protection rules org-wide
- **Repository lifecycle** — archive/unarchive and flip visibility in bulk
- **Issues, teams, org audit log, and secret scanning alerts**
- **Interactive TUI dashboard** — `sgh tui`
- **Built for scale** — concurrent workers, rate-limit tracking, exponential backoff, circuit breaking
- **Scriptable** — `--output table|compact|json`, shell completion for Bash/Zsh/Fish/PowerShell

## Install

```bash
go install github.com/pradyb/sgh-cli@latest
```

Or download a prebuilt binary for Linux, macOS, or Windows from the [releases page](https://github.com/pradyb/sgh-cli/releases) — each release publishes `checksums.txt` alongside the binaries.

Full instructions, including building from source and shell completion: **[docs/installation.md](docs/installation.md)**

## Quick start

**1. Create a token.** A fine-grained PAT is recommended. The [permissions table](docs/authentication.md#repository-permissions) lists exactly what each feature needs — for read-only use you need very little.

**2. Set your environment:**

```bash
export SGH_TOKEN=your_token_here
export SGH_ORG=your-org        # so you can drop --org from every command
```

**3. Check it works:**

```bash
sgh health
sgh whoami
```

**4. Do something useful:**

```bash
# What's open across the org?
sgh pr list --org your-org

# Cut a release branch everywhere — preview first
sgh branch create --org your-org --new Release-1.1 --ref main --dry-run
sgh branch create --org your-org --new Release-1.1 --ref main
```

> **Preview before you commit.** Every write command accepts `--dry-run`. On a tool that acts on every repository at once, that flag is the difference between a bulk operation and a bulk incident.

## Commands

| Group | Commands |
|---|---|
| **Repositories** | `repo` · `clone` · `commit` · `issue` |
| **Git** | `branch` · `tag` · `pr` · `protected-branch` |
| **CI/CD** | `workflow` · `post-release` |
| **Organization** | `org` · `team` · `security` · `audit` |
| **Utilities** | `config` · `tui` · `whoami` · `health` · `shortcuts` · `version` · `completion` |

Most `list` and `view` subcommands have a single-word shortcut — `prl`, `brl`, `wfl`, `rpl`. Run `sgh shortcuts` for the full set.

Full reference with every alias, subcommand, and flag: **[docs/commands.md](docs/commands.md)**

## Documentation

| | |
|---|---|
| [Installation](docs/installation.md) | Install methods, checksums, shell completion |
| [Authentication](docs/authentication.md) | Token types and the permissions each feature needs |
| [Configuration](docs/configuration.md) | Config file, per-owner tokens, repository filtering |
| [Command reference](docs/commands.md) | Every command, flag, and shorthand |
| [Usage examples](docs/examples.md) | Working examples per command group |
| [Advanced usage](docs/advanced.md) | Scripting, concurrency, rate limits |
| [Troubleshooting](docs/troubleshooting.md) | 403s, rate limits, empty results |
| [Development](docs/development.md) | Building, testing, architecture |

## 🔒 Security

To report a security vulnerability, **do not open a public GitHub issue.** See the [Security Policy](SECURITY.md) for responsible disclosure instructions.

Token safety:

- The config file stores tokens in plain text — keep it out of version control
- Prefer fine-grained PATs scoped to the repositories you actually need
- Rotate immediately if you suspect exposure

## 🤝 Contributing

Contributions are welcome. For anything substantial, open an issue first so we can agree on the approach before you write code. See [CONTRIBUTING.md](CONTRIBUTING.md) for the process and [docs/development.md](docs/development.md) for the build and test setup.

## 📄 License

MIT — see [LICENSE](LICENSE).

Copyright (c) 2024 Pradeep Kumar Balakrishnan

## 🙏 Acknowledgments

Built with [Cobra](https://github.com/spf13/cobra), [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Bubbles](https://github.com/charmbracelet/bubbles), [Lipgloss](https://github.com/charmbracelet/lipgloss), [Progressbar](https://github.com/schollz/progressbar), and [Zerolog](https://github.com/rs/zerolog), on top of the [GitHub REST](https://docs.github.com/en/rest) and [GraphQL](https://docs.github.com/en/graphql) APIs.
