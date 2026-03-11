# 🚀 Simple GitHub CLI (sgh-cli)

A powerful command-line tool for managing GitHub repositories at scale. Perform bulk operations on branches, tags, pull requests, protected branches, and more across your entire GitHub organization with a single command.

[![CI](https://github.com/pradyb/sgh-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/pradyb/sgh-cli/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.26.1+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

## 📋 Table of Contents

- [✨ Key Features](#-key-features)
- [🚀 Quick Start](#-quick-start)
- [📦 Installation](#-installation)
- [🔐 Authentication](#-authentication)
- [⚙️ Configuration](#️-configuration)
- [📚 Available Commands](#-available-commands)
- [⚡ Command Shortcuts](#-command-shortcuts)
- [🌟 Usage Examples](#-usage-examples)
- [🏷️ Global Flags](#️-global-flags)
- [🔍 Advanced Usage](#-advanced-usage)
- [⚡ Performance Tips](#-performance-tips)
- [🐛 Troubleshooting](#-troubleshooting)
- [🧪 Testing & Development](#-testing--development)
- [📄 License](#-license)
- [🤝 Contributing](#-contributing)

## ✨ Key Features

- **Bulk Repository Operations**: Manage multiple repositories across entire organizations
- **Advanced Branch & Tag Management**: Create, delete, and manage branches and tags across repos
- **Pull Request Automation**: Create, list, update, and merge pull requests in bulk
- **GitHub Actions / Workflow Runs**: List, view, rerun, and cancel workflow runs across repos with live monitoring
- **Protected Branch Management**: Configure and update branch protection rules
- **Post-Release Workflows**: Automate post-release activities like merging and tagging
- **Team Management**: List teams and members across your organization
- **Repository Operations**: Clone repositories and track commits
- **Flexible Filtering**: Use include/exclude patterns to target specific repositories
- **Concurrent Processing**: Configurable worker threads for fast bulk operations
- **Interactive PR Management**: Interactive terminal UI for pull request operations
- **Flexible Output Modes**: Unified `--output table|compact|json` flag (plus `--compact` / `--json` shorthands)
- **Adaptive Terminal UI**: Tables auto-size to terminal width with colored status indicators
- **Shell Completion**: Built-in completion for Bash, Zsh, Fish, and PowerShell
- **Graceful Error Handling**: Comprehensive error handling with helpful guidance
- **Rate Limit Management**: Built-in rate limiting and retry mechanisms

## 🚀 Quick Start

1. **Set your GitHub token:**
   ```bash
   export GITHUB_TOKEN=your_token_here
   ```

2. **Optionally set default org and worker count:**
   ```bash
   export SGH_ORG=your-org        # avoids repeating --org on every command
   export SGH_WORKERS=10          # override the default 5 workers
   ```

3. **List repositories in your organization:**
   ```bash
   sgh repo list --org your-org
   ```

4. **Create branches across repositories:**
   ```bash
   sgh branch create --org your-org --new feature-branch --ref main
   ```

5. **Bulk PR creation:**
   ```bash
   sgh pr create --org your-org --title "Feature update" --head feature-branch --base main
   ```

## 📦 Installation

### Prerequisites
- **Go 1.24.0 or higher** (for building from source)
- **GitHub Personal Access Token** with appropriate permissions:
  - `repo` - Full repository access
  - `admin:org` - Organization administration (for team operations)
  - `delete_repo` - Repository deletion (if needed)

### Option 1: From Source (Recommended)
```bash
git clone https://github.com/pradyb/sgh-cli.git
cd sgh-cli
go build -o sgh .

# Add to PATH (optional)
# Linux/Mac:
sudo mv sgh /usr/local/bin/
# Windows: Move sgh.exe to a directory in your PATH
```

### Option 2: Go Install
```bash
go install github.com/pradyb/sgh-cli@latest
```

### Option 3: Download Binary
Visit the [releases page](https://github.com/pradyb/sgh-cli/releases) and download the appropriate binary for your platform.

### Verify Installation
```bash
sgh --help
sgh version
```

## 🔐 Authentication

### Create a GitHub Personal Access Token:
1. Go to [GitHub Settings → Developer settings → Personal access tokens](https://github.com/settings/tokens)
2. Click "Generate new token" (classic)
3. Select the required scopes (see Prerequisites above)
4. Copy the token and set it as an environment variable:

**Linux/Mac:**
```bash
export GITHUB_TOKEN=your_token_here
# Add to ~/.bashrc or ~/.zshrc for persistence
echo 'export GITHUB_TOKEN=your_token_here' >> ~/.bashrc
```

**Windows (PowerShell):**
```powershell
$env:GITHUB_TOKEN="your_token_here"
# For persistence, add to PowerShell profile
```

**Windows (Command Prompt):**
```cmd
set GITHUB_TOKEN=your_token_here
```

### Token Requirements
- Must be at least 20 characters long
- Must start with: `ghp_`, `gho_`, `ghu_`, `ghs_`, `ghr_`, or `github_pat_`
- Cannot contain spaces
- Test tokens (starting with 'ghp_test_') are not allowed

## ⚙️ Configuration

### Config File Locations
- **Windows:** `~/sgh.json`
- **Linux:** `~/.config/sgh/sgh.json`
- **Mac:** `~/.config/sgh/sgh.json`

### Configuration Management
```bash
# View current configuration
sgh config list

# Validate configuration for errors
sgh config validate

# Add organization
sgh config add org my-organization

# Add repository to organization
sgh config add repo my-repo --org my-organization

# Add include patterns (only these repos will be processed)
sgh config add pattern api-* --org my-organization --include
sgh config add pattern service-* --org my-organization --include

# Add exclude patterns (these repos will be skipped)
sgh config add pattern legacy-* --org my-organization --exclude
sgh config add pattern deprecated-* --org my-organization --exclude
```

### Sample Configuration File
```json
{
  "organizations": {
    "my-org": {
      "repositories": ["repo1", "repo2"],
      "include_patterns": ["api-*", "service-*"],
      "exclude_patterns": ["legacy-*", "test-*"]
    }
  }
}
```

## 📚 Available Commands

Commands are organized into groups:

### Repository Management

| Command | Alias | Subcommands | Description |
|---------|-------|-------------|-------------|
| `repo` | | `list`, `search` | List and search repositories for an organization |
| `clone` | | - | Clone multiple repositories at once |
| `commit` | `ci` | `list` | Track and list commits across repositories |

### Git Operations

| Command | Alias | Subcommands | Description |
|---------|-------|-------------|-------------|
| `branch` | `br` | `list`, `create`, `delete` | List, create, and delete branches across repositories |
| `tag` | `tg` | `list`, `create`, `delete` | List, create, and delete tags across repositories |
| `pr` | | `create`, `list`, `view`, `review`, `update`, `merge` | Create, list, view, review, update, and merge pull requests |
| `pb` | | `list`, `update`, `delete` | Manage protected branch settings |

### CI/CD & Release

| Command | Alias | Subcommands | Description |
|---------|-------|-------------|-------------|
| `workflow` | `wf` | `list`, `view`, `rerun`, `cancel` | Manage GitHub Actions workflow runs |
| `post-release` | | - | Automate post-release workflows |

### Organization

| Command | Alias | Subcommands | Description |
|---------|-------|-------------|-------------|
| `team` | | `list` | List teams and members |

### Utilities

| Command | Alias | Subcommands | Description |
|---------|-------|-------------|-------------|
| `config` | `cfg` | `list`, `validate`, `add`, `set` | Manage and validate CLI configuration |
| `health` | | - | Check system health and connectivity |
| `shortcuts` | | - | List available command shortcuts |
| `version` | | - | Display version information |
| `completion` | | `bash`, `zsh`, `fish`, `powershell` | Generate shell completion scripts |

### Command Details

**Repository Management:**
- `repo list --org <org> [--all]` - List repositories for an organization
- `repo search --org <org> --query <text> [--language <lang>] [--topic <topic>]` - Search repositories
- `clone --org <org> [--branch <branch>]` - Clone multiple repositories
- `commit list --org <org> [--days <n>]` - Track commits across repositories

**Branch & Tag Operations:**
- `branch list --org <org> [--filter <pattern>]` - List branches across repos with optional name filter (regex)
- `branch create --org <org> --new <name> --ref <ref>` - Create branches across repos
- `branch delete --org <org> --branch <name>` - Delete branches across repos
- `tag list --org <org>` - List tags across repos
- `tag create --org <org> --tag <name> --head <ref> --message <msg>` - Create tags
- `tag delete --org <org> --tag <name>` - Delete tags

**Pull Request Workflows:**
- `pr create --org <org> --title <title> --head <branch> --base <branch>` - Create PRs
- `pr list --org <org> [--all-status] [--sort repo|title|author|status]` - List pull requests
- `pr list --org <org> --label <name> --since 2024-01-01` - Filter by label and date
- `pr view --org <org> --repo <repo> --pr <num>` - View PR details, files, checks, reviews
- `pr review --org <org> --repo <repo> --pr <num> --event approve` - Review a PR
- `pr update --org <org> --repo <repo> --pr <number> --action <close|open>` - Update PRs
- `pr list --interactive` - Interactive PR management with Bubble Tea UI

**Workflow Runs (GitHub Actions):**
- `workflow list --org <org>` - List workflow runs across repositories
- `workflow list --org <org> --running` - Show only in-progress runs
- `workflow list --org <org> --failed --branch main` - Show failed runs on a branch
- `workflow list --org <org> --workflow "CI Build"` - Filter by workflow name
- `workflow view --org <org> --repo <repo> --run <id>` - View run details with jobs & steps
- `workflow view --org <org> --repo <repo> --run <id> --watch` - Live monitor until completion
- `workflow rerun --org <org> --repo <repo> --run <id>` - Re-trigger a workflow run
- `workflow cancel --org <org> --repo <repo> --run <id>` - Cancel an in-progress run

**Protected Branch Management:**
- `pb list --org <org> --branch <branch>` - List protection settings
- `pb update --org <org> --branch <branch> [options]` - Update protection rules
- `pb delete --org <org> --branch <branch>` - Remove protection

**Organization & Configuration:**
- `team list --org <org>` - List teams and members
- `config list` - Show current configuration
- `config validate` - Check configuration for errors
- `config add <key> <value>` - Add configuration

### ⚡ Command Shortcuts

For faster typing, single-word shortcuts are available for common `list` and `view` subcommands. Run `sgh shortcuts` to see them all.

| Shortcut | Expands To | Shortcut | Expands To |
|----------|------------|----------|------------|
| `rpl` | `repo list` | `rps` | `repo search` |
| `prl` | `pr list` | `prv` | `pr view` |
| `brl` | `branch list` | `tgl` | `tag list` |
| `wfl` | `workflow list` | `wfv` | `workflow view` |
| `pbl` | `pb list` | `cil` | `commit list` |
| `tml` | `team list` | | |

Each shortcut supports the same flags as the full command:

```bash
sgh prl --org my-org --author john-doe        # same as: sgh pr list --org my-org --author john-doe
sgh wfl --org my-org --running                # same as: sgh workflow list --org my-org --running
sgh brl --org my-org --filter "Release-"      # same as: sgh branch list --org my-org --filter "Release-"
sgh rps --org my-org --query "api"            # same as: sgh repo search --org my-org --query "api"
```

## 🌟 Usage Examples

### Branch Management
```bash
# List branches across all repos
sgh branch list --org my-org

# List branches matching a pattern (regex)
sgh branch list --org my-org --filter "Release-"
sgh branch list --org my-org --filter "feature/" --repo my-app

# Create a new release branch across all repos
sgh branch create --org my-org --new Release-1.1 --ref Release-1.0

# Create hotfix branch for specific repositories
sgh branch create --org my-org --new hotfix-branch --ref main --repo critical-app --repo important-service

# Delete old branches
sgh branch delete --org my-org --branch old-feature --repo legacy-app
```

### Tag Operations
```bash
# List tags across all repos
sgh tag list --org my-org

# List tags for specific repos
sgh tag list --org my-org --repo app1 --repo app2

# Create release tags across repositories
sgh tag create --org my-org --tag v1.0.0 --head Release-1.0 --message 'Release v1.0.0'

# Create tags for specific repositories
sgh tag create --org my-org --tag v2.1.0 --head main --message 'Version 2.1.0' --repo app1 --repo app2

# Delete old tags
sgh tag delete --org my-org --tag old-version --repo legacy-app
```

### Pull Request Automation
```bash
# Create PRs across multiple repositories
sgh pr create --org my-org --title "Security Update" --body "Update dependencies" --head security-patch --base main

# List PRs for specific repositories
sgh pr list --org my-org --repo app1 --repo app2 --base main --all-status

# List PRs with filters
sgh pr list --org my-org --author john-doe --assignee jane-doe --last 10

# Filter by label and creation date
sgh pr list --org my-org --label bug --since 2024-01-01

# View detailed PR information (files, checks, reviews)
sgh pr view --org my-org --repo my-app --pr 42

# Review a pull request
sgh pr review --org my-org --repo my-app --pr 42 --event approve
sgh pr review --org my-org --repo my-app --pr 42 --event request_changes --body "Please fix the tests"
sgh pr review --org my-org --repo my-app --pr 42 --event comment --body "Looks good overall"

# Update PR status
sgh pr update --org my-org --repo my-app --pr 123 --action close

# Interactive PR management
sgh pr list --org my-org --interactive
```

### Protected Branch Management
```bash
# List protected branch settings
sgh pb list --org my-org --branch main

# Update protection rules
sgh pb update --org my-org --branch main --repo my-app --add-bypass-user admin --add-push-user ci-bot

# Configure protection for all repositories
sgh pb update --org my-org --branch main --require-reviews --dismiss-stale-reviews
```

### Post-Release Workflows
```bash
# Complete post-release workflow with tagging
sgh post-release --org my-org --base main --head Release-1.0 --create-tag --title "Release 1.0"

# Post-release for specific repositories
sgh post-release --org my-org --base develop --head feature-complete --repo service1 --repo service2

# Exclude specific repositories from post-release
sgh post-release --org my-org --base main --head Release-1.0 --exclude-repos legacy-app --exclude-repos deprecated-service
```

### Repository Operations
```bash
# List all repositories
sgh repo list --org my-org

# Search repositories by name or description
sgh repo search --org my-org --query "api"

# Search by language and topic
sgh repo search --org my-org --language go --topic microservice

# Clone repositories with specific branch
sgh clone --org my-org --branch develop

# Track recent commits
sgh commit list --org my-org --days 7 --details --include-merge-commits
```

### Workflow Runs (GitHub Actions)
```bash
# List all recent workflow runs across repos
sgh workflow list --org my-org

# Show only running workflows
sgh workflow list --org my-org --running

# Show only failed workflows on a specific branch
sgh workflow list --org my-org --failed --branch main

# Filter by workflow name (partial match)
sgh workflow list --org my-org --workflow "CI Build"

# Sort by status
sgh workflow list --org my-org --sort status

# View detailed run info with jobs and steps
sgh workflow view --org my-org --repo my-app --run 123456789

# Watch a run live until it completes (polls every 10s)
sgh workflow view --org my-org --repo my-app --run 123456789 --watch

# Watch with a custom polling interval
sgh workflow view --org my-org --repo my-app --run 123456789 --watch --interval 5

# Rerun a failed workflow
sgh workflow rerun --org my-org --repo my-app --run 123456789

# Cancel a running workflow
sgh workflow cancel --org my-org --repo my-app --run 123456789
```

### Security (Secret Scanning Alerts)
```bash
# List all secret scanning alerts across repositories
sgh security list --org my-org

# List only open alerts
sgh security list --org my-org --state open

# List resolved alerts
sgh security list --org my-org --state resolved

# Filter by secret type
sgh security list --org my-org --secret-type aws_access_key
sgh security list --org my-org --secret-type github_token

# List alerts for specific repositories
sgh security list --org my-org -r api-service -r web-app

# View detailed alert information
sgh security view --org my-org -r my-app --alert 1

# View alert in JSON format
sgh security view --org my-org -r my-app --alert 5 --json

# Resolve an alert as false positive
sgh security update --org my-org -r my-app --alert 1 --state resolved --resolution false_positive

# Resolve with a comment
sgh security update --org my-org -r my-app --alert 2 --state resolved --resolution revoked --comment "Key has been rotated"

# Reopen a resolved alert
sgh security update --org my-org -r my-app --alert 3 --state open

# Mark as used in tests
sgh security update --org my-org -r my-app --alert 4 --state resolved --resolution used_in_tests

# Preview changes with dry-run
sgh security update --org my-org -r my-app --alert 1 --state resolved --resolution false_positive --dry-run
```

### Team Management
```bash
# List all teams
sgh team list --org my-org

# List specific team with all members
sgh team list --org my-org --team developers --all-members

# Get team details with member count
sgh team list --org my-org --members 100
```

### Configuration Management
```bash
# View current configuration
sgh config list

# Validate config file for errors
sgh config validate

# Add organization to config
sgh config add org my-org

# Add repository patterns
sgh config add pattern api-* --org my-org --include
sgh config add pattern legacy-* --org my-org --exclude
```

## 🏷️ Global Flags

- `-h, --help` - Show help information
- `-o, --org string` - Organization name (env: `SGH_ORG`, required for most commands)
- `-v, --verbose` - Enable verbose output
- `-L, --log-response` - Log HTTP responses for debugging
- `-w, --workers int` - Number of concurrent workers (default: 5, env: `SGH_WORKERS`)
- `-O, --output string` - Output format: `table` (default), `compact`, or `json`
- `-C, --compact` - Shorthand for `--output compact` (tab-separated, pipe-friendly)
- `-J, --json` - Shorthand for `--output json` (structured JSON for scripting)
- `--dry-run` - Preview what would be changed without executing
- `--no-color` - Disable colored output (env: `NO_COLOR`)
- `--limit int` - Limit the **total** number of items returned in the global output (0 = no limit)

> **Tip:** Set `SGH_ORG` and `SGH_WORKERS` environment variables to avoid repeating common flags.
> **Note on Limits:** For multi-repo commands, use `--last` to limit how many items are fetched *per repository* from the API, and `--limit` to truncate the *final combined output*.

### Per-Command Flags

Some list commands support additional flags:

- `--sort <field>` - Sort table output (available on `pr`, `branch`, `tag`, `issue`, `security`, and `workflow` list commands)
- `--last <count>` - Number of items to fetch **per repository** from the GitHub API (on `pr`, `workflow`, and `issue` list commands)
- `--filter <pattern>` - Branch name filter with regex support (on `branch list`)
- `--workflow <name>` - Workflow name filter with partial match (on `workflow list`)
- `--label <name>`, `--since <YYYY-MM-DD>` - PR filters (on `pr list`)
- `--running`, `--queued`, `--failed` - Quick status filters (on `workflow list`)
- `--watch`, `--interval` - Live monitoring (on `workflow view`)

## 🔍 Advanced Usage

### Repository Filtering
Use include/exclude patterns to target specific repositories:

```bash
# Target only API services
sgh branch create --org my-org --new feature --ref main --repo api-*

# Exclude legacy applications
sgh pr create --org my-org --title "Update" --head feature --base main --exclude-repos legacy-*
```

### Output Modes
Use `--output`, `--compact`, or `--json` for scripting:

```bash
# Unified --output flag
sgh pr list --org my-org --output compact | grep "open"
sgh repo list --org my-org --output json | jq '.[].name'

# Shorthand flags (equivalent to --output compact / --output json)
sgh pr list --org my-org -C | grep "open"
sgh workflow list --org my-org --running -C | awk '{print $1, $4}'
sgh workflow list --org my-org -J | jq '.[] | select(.status == "failure")'
```

### Shell Completion
Generate completion scripts for your shell:

```bash
# Bash
sgh completion bash > /etc/bash_completion.d/sgh

# Zsh
sgh completion zsh > "${fpath[1]}/_sgh"

# Fish
sgh completion fish > ~/.config/fish/completions/sgh.fish

# PowerShell
sgh completion powershell | Out-String | Invoke-Expression
```

### Concurrent Processing
Adjust worker count for optimal performance:

```bash
# Use more workers for faster processing
sgh clone --org large-org --workers 10

# Use fewer workers to avoid rate limiting
sgh pr create --org my-org --title "Update" --head feature --base main --workers 2
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GITHUB_TOKEN` | GitHub Personal Access Token (**required**) | - |
| `SGH_ORG` | Default organization name | - |
| `SGH_WORKERS` | Default number of concurrent workers | `5` |
| `SGH_TIMEOUT` | HTTP client timeout (e.g. `60s`) | `30s` |
| `SGH_VERBOSE` | Enable verbose output (`true`/`false`) | `false` |
| `SGH_LOG_RESPONSE` | Log HTTP responses (`true`/`false`) | `false` |
| `NO_COLOR` | Disable colored output | - |

### Configuration Patterns
Set up organization-specific configurations:

```bash
# Configure default organization
sgh config add org my-primary-org

# Add include patterns for microservices
sgh config add pattern service-* --org my-primary-org --include
sgh config add pattern api-* --org my-primary-org --include

# Exclude archived repositories
sgh config add pattern archived-* --org my-primary-org --exclude
```

## ⚡ Performance Tips

- **Adjust worker count** based on your rate limits and network capacity (default: 5)
- **Use specific repository filters** to avoid processing unnecessary repos
- **Enable verbose mode** (`-v`) for detailed operation logs and debugging
- **Use configuration files** to avoid repeating common parameters
- **Monitor rate limits** - GitHub has API rate limits (5,000 requests/hour for authenticated users)
- **Use exclude patterns** to skip repositories you don't need to process

### Rate Limit Management
```bash
# Check current rate limit status
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/rate_limit

# Reduce workers if hitting limits
sgh pr create --org my-org --workers 2 --title "Update"

# Use smaller batches for large operations
sgh branch create --org my-org --new feature --ref main --repo specific-repo
```

## 🐛 Troubleshooting

### Common Issues

**Rate Limiting:**
```bash
# Check rate limit status
curl -H "Authorization: token $GITHUB_TOKEN" https://api.github.com/rate_limit

# Reduce worker count to avoid rate limits
sgh pr create --org my-org --workers 2 --title "Update"
```

**Permission Errors:**
- Ensure your GitHub token has the required scopes (`repo`, `admin:org`)
- Verify you have admin access to the organization/repositories
- Check if the organization has SSO enabled and authorize your token

**Network Issues:**
```bash
# Enable response logging for debugging
sgh repo list --org my-org --log-response --verbose
```

**Configuration Issues:**
```bash
# Check current configuration
sgh config list

# Validate configuration for errors
sgh config validate

# Reset configuration
rm ~/.config/sgh/sgh.json  # Linux/Mac
rm ~/sgh.json              # Windows
```

**Command Not Found:**
```bash
# Verify installation
which sgh  # Linux/Mac
where sgh  # Windows

# Add to PATH if needed
export PATH=$PATH:/path/to/sgh  # Linux/Mac
```

**Token Validation Issues:**
- Token must be at least 20 characters long
- Token must start with: `ghp_`, `gho_`, `ghu_`, `ghs_`, `ghr_`, or `github_pat_`
- Token cannot contain spaces
- Test tokens (starting with 'ghp_test_') are not allowed

## 🧪 Testing & Development

### Running Tests

**Run all tests in the project:**
```bash
go test ./...
```

**Run tests with verbose output:**
```bash
go test ./... -v
```

**Run tests for a specific package:**
```bash
go test ./internal/config
go test ./pkg/config
```

### Test Coverage

**Basic coverage percentage:**
```bash
go test ./internal/config -cover
```

**Generate detailed coverage profile:**
```bash
go test ./internal/config -coverprofile=coverage.out
```

**View function-level coverage details:**
```bash
go tool cover -func=coverage.out
```

**Generate interactive HTML coverage report:**
```bash
go tool cover -html=coverage.out -o coverage.html
# Open coverage.html in your browser for visual coverage report
```

**Coverage for all packages with tests:**
```bash
go test ./... -coverprofile=fullcoverage.out
go tool cover -func=fullcoverage.out
go tool cover -html=fullcoverage.out -o full-coverage.html
```

### Development Guidelines

- Maintain **>80% test coverage** for core packages
- Add tests for new features and bug fixes
- Run tests before submitting pull requests
- Use the HTML coverage report to identify untested code paths
- Follow Go best practices and conventions

### Project Structure
```
sgh-cli/
├── cmd/                    # Command implementations
│   ├── branch/            # Branch management commands
│   ├── clone/             # Repository cloning
│   ├── commit/            # Commit tracking
│   ├── config/            # Configuration management
│   ├── health/            # Health check command
│   ├── postrelease/       # Post-release automation
│   ├── pr/                # Pull request operations
│   ├── protectedbranch/   # Protected branch management
│   ├── repo/              # Repository operations
│   ├── tag/               # Tag management
│   ├── team/              # Team management
│   ├── version/           # Version information
│   └── workflow/          # GitHub Actions workflow management
├── internal/              # Internal packages
│   ├── async/             # Async job queue & worker pool
│   ├── circuitbreaker/    # Circuit breaker pattern
│   ├── client/            # HTTP client with resilience (retry, rate limit)
│   ├── config/            # Configuration handling
│   ├── model/             # Data models (repos, PRs, workflows, etc.)
│   ├── processor/         # Bulk repository operation processor
│   ├── ratelimit/         # GitHub API rate limiting
│   ├── retry/             # Exponential backoff retry
│   └── service/           # GitHub REST & GraphQL API services
├── pkg/                   # Public packages
│   ├── apperrors/         # Application error types
│   ├── branch/            # Branch business logic (list, create, delete)
│   ├── config/            # Config helpers (add, set, save)
│   ├── context/           # Global application context
│   ├── logger/            # Structured logging (zerolog)
│   ├── pr/prompt/         # Interactive PR selection (Bubble Tea)
│   ├── repo/              # Repository operations
│   ├── tag/               # Tag business logic (list, create, delete)
│   ├── ui/                # Table rendering, colors, progress bars
│   └── workflow/          # Workflow business logic
└── utils/                 # Utility functions
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

Copyright (c) 2024 Pradeep Kumar Balakrishnan

## 🤝 Contributing

We welcome contributions! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

### Contributing Guidelines

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Setup

1. Clone the repository
2. Install dependencies: `go mod download`
3. Run tests: `go test ./...`
4. Build the binary: `go build -o sgh .`

## 🙏 Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI structure and shell completion
- Uses [GitHub REST API](https://docs.github.com/en/rest) and [GraphQL API](https://docs.github.com/en/graphql)
- Interactive UI powered by [Bubble Tea](https://github.com/charmbracelet/bubbletea) and [Bubbles](https://github.com/charmbracelet/bubbles)
- Table rendering and styling with [Lipgloss](https://github.com/charmbracelet/lipgloss)
- Progress bars with [Progressbar](https://github.com/schollz/progressbar)
- Logging with [Zerolog](https://github.com/rs/zerolog)
- Inspired by the need for bulk GitHub operations in large organizations

---

**Happy coding! 🚀**

For support, please open an issue on [GitHub](https://github.com/pradyb/sgh-cli/issues).
