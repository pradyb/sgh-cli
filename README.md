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
- **Advanced Branch & Tag Management**: Create, rename, delete, and filter branches and tags across repos
- **Pull Request Automation**: Create, list, view, review, update, close, reopen, and merge pull requests in bulk
- **GitHub Actions / Workflow Runs**: List, view, rerun, cancel, and dispatch workflow runs across repos with live monitoring
- **Protected Branch Management**: Configure and update branch protection rules
- **Repository Lifecycle**: Archive/unarchive repos and toggle visibility (public/private) in bulk
- **Issue Management**: Create and list issues across repositories
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
- **Linux/Mac:** `~/.config/sgh/sgh.json`

### Configuration Management
```bash
# View current configuration
sgh config list

# Validate configuration for errors
sgh config validate

# Add organization
sgh config add org my-org

# Add specific repositories for fuzzy name matching (used with -r flag)
sgh config add repo my-repo --org my-org

# Add include patterns — only repos matching these will be processed
sgh config add pattern "^api-" --org my-org --include
sgh config add pattern "^service-" --org my-org --include

# Add exclude patterns — repos matching these are always skipped
sgh config add pattern "legacy-.*" --org my-org --exclude
sgh config add pattern ".*-deprecated$" --org my-org --exclude

# Set tagger identity (used by tag create)
sgh config set tagger-name "John Doe" --org my-org
sgh config set tagger-email "john@example.com" --org my-org
```

### Sample Configuration File
```json
{
  "no_of_workers": 5,
  "organizations": [
    {
      "name": "my-org",
      "repositories": ["api-gateway", "service-auth", "service-billing"],
      "repo_patterns": {
        "include": ["^api-", "^service-"],
        "exclude": ["legacy-.*", ".*-deprecated$", ".*-archive$"]
      },
      "pull_request_assignees": ["jane-doe", "john-smith"],
      "tagger": {
        "name": "Release Bot",
        "email": "release@my-org.com"
      }
    }
  ]
}
```

> **Note:** Patterns are **Go regular expressions**, not globs.  
> Use `^api-` (not `api-*`) to match repos starting with `api-`.

### Repository Include / Exclude Filtering

When no `--repo` (`-r`) flag is given, `sgh` uses the `repo_patterns` in config to decide which repositories to process. The rules are evaluated in this order:

| Priority | Condition | Result |
|----------|-----------|--------|
| 1 | No `include` **and** no `exclude` patterns configured | Include **all** repos |
| 2 | Repo matches any `exclude` pattern | **Always excluded** (even if it also matches an include pattern) |
| 3 | `include` patterns are configured and repo matches at least one | Included |
| 3 | `include` patterns are configured but repo matches **none** | Excluded |
| 4 | Only `exclude` patterns configured (no include), repo doesn't match | Included |

**Examples:**

```
Config: include=[^api-, ^service-]  exclude=[.*-legacy$]

  api-gateway         → included   (matches include)
  service-auth        → included   (matches include)
  api-legacy          → EXCLUDED   (matches exclude — exclude always wins)
  web-frontend        → EXCLUDED   (include is active but no match)
  random-tool         → EXCLUDED   (include is active but no match)
```

```
Config: include=[]  exclude=[.*-archive$, .*-deprecated$]

  api-gateway         → included   (no include filter, not excluded)
  old-app-archive     → EXCLUDED   (matches exclude)
  service-deprecated  → EXCLUDED   (matches exclude)
  web-frontend        → included   (no include filter, not excluded)
```

#### The `repositories` list vs `repo_patterns`

| Field | Purpose |
|-------|---------|
| `repositories` | Auto-populated fuzzy-match dictionary for the `-r` flag. Allows short/partial names like `-r api` to resolve to `api-gateway`. Not used for filtering when no `-r` is given. |
| `repo_patterns.include` / `repo_patterns.exclude` | Controls which repos are selected when **no** `-r` flag is provided. Applied as regex. Exclude always wins. |

> **Important:** Even when you explicitly pass repos via `-r`, the `exclude` patterns from config still apply.  
> If a repo is in your `exclude` list, it will be skipped even if named directly with `-r`.

## 📚 Available Commands

Commands are organized into groups:

### Repository Management

| Command | Alias | Subcommands | Description |
|---------|-------|-------------|-------------|
| `repo` | | `list`, `search`, `archive`, `visibility` | List, search, archive, and manage repository visibility |
| `clone` | | - | Clone multiple repositories at once |
| `commit` | `ci` | `list` | Track and list commits across repositories |

### Git Operations

| Command | Alias | Subcommands | Description |
|---------|-------|-------------|-------------|
| `branch` | `br` | `list`, `create`, `rename`, `delete` | List, create, rename, and delete branches across repositories |
| `tag` | `tg` | `list`, `create`, `delete` | List, create, and delete tags across repositories |
| `issue` | | `list`, `view`, `create` | List, view, and create issues across repositories |
| `pr` | | `create`, `list`, `view`, `review`, `update`, `merge`, `close`, `reopen` | Full pull request lifecycle management |
| `protected-branch` | `pb` | `list`, `update`, `delete` | Manage protected branch settings |

### CI/CD & Release

| Command | Alias | Subcommands | Description |
|---------|-------|-------------|-------------|
| `workflow` | `wf` | `list`, `view`, `rerun`, `cancel`, `dispatch` | Manage GitHub Actions workflow runs |
| `post-release` | | - | Automate post-release workflows |

### Organization

| Command | Alias | Subcommands | Description |
|---------|-------|-------------|-------------|
| `org` | `orl` | `list` | List all GitHub organizations the token belongs to |
| `team` | `tml` | `list` | List teams and members |

### Utilities

| Command | Alias | Subcommands | Description |
|---------|-------|-------------|-------------|
| `config` | `cfg` | `list`, `validate`, `add`, `set`, `reset` | Manage and validate CLI configuration |
| `health` | | - | Check system health and connectivity (`--json` for structured output) |
| `shortcuts` | | - | List available command shortcuts |
| `version` | | - | Display version information (`--short` for version string only) |
| `completion` | | `bash`, `zsh`, `fish`, `powershell` | Generate shell completion scripts |

### Command Details

**Repository Management:**
- `repo list --org <org> [--all]` - List repositories for an organization
- `repo search --org <org> --query <text> [--language <lang>] [--topic <topic>]` - Search repositories
- `repo archive --org <org> [-r <repo>...] [--unarchive]` - Archive or unarchive repositories in bulk
- `repo visibility --org <org> [-r <repo>...] --visibility public|private` - Set repository visibility
- `clone --org <org> [--branch <branch>]` - Clone multiple repositories
- `commit list --org <org> [--days <n>]` - Track commits across repositories

**Branch & Tag Operations:**
- `branch list --org <org> [--filter <pattern>]` - List branches with optional regex filter
- `branch create --org <org> --new <name> --ref <ref>` - Create branches across repos
- `branch rename --org <org> --old <name> --new <name>` - Rename a branch across repos
- `branch delete --org <org> --branch <name>` - Delete branches across repos
- `tag list --org <org> [--filter <pattern>]` - List tags with optional regex filter
- `tag create --org <org> --tag <name> --head <ref> --message <msg>` - Create tags
- `tag delete --org <org> --tag <name>` - Delete tags

**Issue Workflows:**
- `issue list --org <org>` - List open issues across repositories
- `issue view --org <org> -r <repo> --issue <num>` - View issue details
- `issue create --org <org> -r <repo> --title <title> [--body <body>] [--assignee <user>] [--labels <label>]` - Create an issue

**Pull Request Workflows:**
- `pr create --org <org> --title <title> --head <branch> --base <branch>` - Create PRs
- `pr list --org <org> [--all] [--sort repo|title|author|status]` - List pull requests (default: open only)
- `pr list --org <org> --label <name> --since 2024-01-01` - Filter by label and date
- `pr view --org <org> -r <repo> --pr <num>` - View PR details, files, checks, reviews
- `pr review --org <org> -r <repo> --pr <num> --event approve|request_changes|comment` - Review a PR
- `pr update --org <org> -r <repo> --pr <num> --state open|closed` - Update PR state
- `pr close --org <org> -r <repo> --pr <num>` - Close a pull request
- `pr reopen --org <org> -r <repo> --pr <num>` - Reopen a pull request
- `pr list --interactive` - Interactive PR management with Bubble Tea UI

**Workflow Runs (GitHub Actions):**
- `workflow list --org <org>` - List workflow runs across repositories
- `workflow list --org <org> --running` - Show only in-progress runs
- `workflow list --org <org> --failed --branch main` - Show failed runs on a branch
- `workflow list --org <org> --workflow "CI Build"` - Filter by workflow name
- `workflow view --org <org> -r <repo> --run <id>` - View run details with jobs & steps
- `workflow view --org <org> -r <repo> --run <id> --watch` - Live monitor until completion
- `workflow rerun --org <org> -r <repo> --run <id>` - Re-trigger a workflow run
- `workflow cancel --org <org> -r <repo> --run <id>` - Cancel an in-progress run
- `workflow dispatch --org <org> --workflow <file> --ref <branch> [--input key=value]` - Trigger `workflow_dispatch` event

**Protected Branch Management:**
- `protected-branch list --org <org> [--branch <branch>]` - List protection settings
- `protected-branch update --org <org> --branch <branch> [--lock] [--remove-status-checks]` - Update protection rules
- `protected-branch delete --org <org> --branch <branch>` - Remove protection

**Organization & Configuration:**
- `org list` - List all GitHub organizations the token belongs to (no `--org` needed)
- `team list --org <org> [--team <name>] [--all]` - List teams and all their members
- `config list` - Show current configuration
- `config validate` - Check configuration for errors
- `config add <key> <value>` - Add configuration
- `config reset [--org <name>] [--force]` - Remove one or all organizations from config

### ⚡ Command Shortcuts

For faster typing, single-word shortcuts are available for common `list` and `view` subcommands. Run `sgh shortcuts` to see them all.

| Shortcut | Expands To | Shortcut | Expands To |
|----------|------------|----------|------------|
| `rpl` | `repo list` | `rps` | `repo search` |
| `rpa` | `repo archive` | `rpv` | `repo visibility` |
| `orl` | `org list` | `isl` | `issue list` |
| `isv` | `issue view` | `isc` | `issue create` |
| `prl` | `pr list` | `prv` | `pr view` |
| `prc` | `pr create` | `prx` | `pr close` |
| `brl` | `branch list` | `brc` | `branch create` |
| `brr` | `branch rename` | `brd` | `branch delete` |
| `tgl` | `tag list` | `tgc` | `tag create` |
| `tgd` | `tag delete` | `pbl` | `protected-branch list` |
| `wfl` | `workflow list` | `wfv` | `workflow view` |
| `wfd` | `workflow dispatch` | `cil` | `commit list` |
| `tml` | `team list` | `secl` | `security list` |

Each shortcut supports the same flags as the full command:

```bash
sgh orl                                       # same as: sgh org list
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

# Rename a branch across all repos
sgh branch rename --org my-org --old master --new main

# Delete old branches
sgh branch delete --org my-org --branch old-feature --repo legacy-app
```

### Tag Operations
```bash
# List tags across all repos
sgh tag list --org my-org

# List tags matching a pattern (regex)
sgh tag list --org my-org --filter "^v1\."

# List tags for specific repos
sgh tag list --org my-org --repo app1 --repo app2

# Create release tags across repositories
sgh tag create --org my-org --tag v1.0.0 --head Release-1.0 --message 'Release v1.0.0'

# Create tags for specific repositories
sgh tag create --org my-org --tag v2.1.0 --head main --message 'Version 2.1.0' --repo app1 --repo app2

# Delete old tags
sgh tag delete --org my-org --tag old-version --repo legacy-app
```

### Issue Management
```bash
# List open issues across all repos
sgh issue list --org my-org

# List issues for specific repos
sgh issue list --org my-org -r my-app -r other-service

# View a specific issue
sgh issue view --org my-org -r my-app --issue 42

# Create an issue
sgh issue create --org my-org -r my-app --title "Bug: login fails" --body "Steps to reproduce..." --assignee john-doe --labels bug,high-priority
```

### Pull Request Automation
```bash
# Create PRs across multiple repositories
sgh pr create --org my-org --title "Security Update" --body "Update dependencies" --head security-patch --base main

# List PRs for specific repositories (open only by default)
sgh pr list --org my-org --repo app1 --repo app2 --base main

# List all PRs including closed and merged
sgh pr list --org my-org --repo app1 --repo app2 --all

# List PRs with filters
sgh pr list --org my-org --author john-doe --assignee jane-doe --last 10

# Filter by label and creation date
sgh pr list --org my-org --label bug --since 2024-01-01

# View detailed PR information (files, checks, reviews)
sgh pr view --org my-org -r my-app --pr 42

# Review a pull request
sgh pr review --org my-org -r my-app --pr 42 --event approve
sgh pr review --org my-org -r my-app --pr 42 --event request_changes --body "Please fix the tests"
sgh pr review --org my-org -r my-app --pr 42 --event comment --body "Looks good overall"

# Update PR state
sgh pr update --org my-org -r my-app --pr 123 --state closed
sgh pr update --org my-org -r my-app --pr 123 --state open

# Close or reopen a PR with dedicated shortcuts
sgh pr close --org my-org -r my-app --pr 123
sgh pr reopen --org my-org -r my-app --pr 123

# Interactive PR management
sgh pr list --org my-org --interactive
```

### Protected Branch Management
```bash
# List all protected branches (all repos in org)
sgh protected-branch list --org my-org

# List protected branches matching a specific name
sgh protected-branch list --org my-org --branch main

# Update protection rules
sgh protected-branch update --org my-org --branch main -r my-app --add-bypass-user admin --add-push-user ci-bot

# Lock the protected branch and remove all required status checks
sgh protected-branch update --org my-org --branch main -r my-app --lock --remove-status-checks
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

# Archive old repositories in bulk
sgh repo archive --org my-org -r old-service -r legacy-api

# Unarchive a repository
sgh repo archive --org my-org -r old-service --unarchive

# Change repository visibility
sgh repo visibility --org my-org -r my-repo --visibility private
sgh repo visibility --org my-org -r internal-tool --visibility public

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
sgh workflow view --org my-org -r my-app --run 123456789

# Watch a run live until it completes (polls every 10s)
sgh workflow view --org my-org -r my-app --run 123456789 --watch

# Watch with a custom polling interval
sgh workflow view --org my-org -r my-app --run 123456789 --watch --interval 5

# Rerun a failed workflow
sgh workflow rerun --org my-org -r my-app --run 123456789

# Cancel a running workflow
sgh workflow cancel --org my-org -r my-app --run 123456789

# Trigger a workflow_dispatch event across all repos
sgh workflow dispatch --org my-org --workflow deploy.yml --ref main

# Trigger with input parameters
sgh workflow dispatch --org my-org -r app1 -r app2 --workflow release.yml --ref main --input env=production --input dry_run=false
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

### Organization Management
```bash
# List all organizations the token belongs to (no --org flag needed)
sgh org list
sgh orl

# JSON output
sgh orl -J

# Limit results
sgh orl --limit 5
```

### Team Management
```bash
# List all teams
sgh team list --org my-org

# List specific team with all members
sgh team list --org my-org --team developers --all

# Get team details with member count
sgh team list --org my-org --members 100
```

### Configuration Management
```bash
# View current configuration (includes pattern rules and repo list)
sgh config list

# Validate config file for errors (regex syntax, duplicate orgs, etc.)
sgh config validate

# Add organization to config
sgh config add org my-org

# Add specific repos to the fuzzy-match dictionary (used with -r flag)
sgh config add repo api-gateway --org my-org
sgh config add repo service-auth --org my-org

# Add include patterns — only repos matching these are selected (regex)
sgh config add pattern "^api-" --org my-org --include
sgh config add pattern "^service-" --org my-org --include

# Add exclude patterns — these repos are always skipped (regex, wins over include)
sgh config add pattern ".*-legacy$" --org my-org --exclude
sgh config add pattern ".*-archive$" --org my-org --exclude

# Set tagger identity for tag commands
sgh config set tagger-name "Release Bot" --org my-org
sgh config set tagger-email "releases@my-org.com" --org my-org

# Remove a specific organization from config (prompts for confirmation)
sgh config reset --org my-org

# Remove all organizations from config (skip confirmation prompt)
sgh config reset --force
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
- `--filter <pattern>` - Name filter with regex support (on `branch list` and `tag list`)
- `--all` - Include closed/merged items (on `pr list`) or all members (on `team list`)
- `--workflow <name>` - Workflow name filter with partial match (on `workflow list`)
- `--label <name>`, `--since <YYYY-MM-DD>` - PR filters (on `pr list`)
- `--running`, `--queued`, `--failed` - Quick status filters (on `workflow list`)
- `--branch <name>` - Filter by branch name (on `workflow list`)
- `--watch`, `--interval` - Live monitoring (on `workflow view`)
- `--input key=value` - Workflow input parameters, repeatable (on `workflow dispatch`)
- `--short` - Print version string only (on `version`)
- `--json` - Structured JSON output (on `health`)

## 🔍 Advanced Usage

### Repository Filtering

There are two complementary ways to control which repositories are processed:

#### 1. Config-based (persistent, automatic)
Set `repo_patterns` in config once and every command respects it automatically:

```bash
# Only process repos matching these patterns (regex)
sgh config add pattern "^api-" --org my-org --include
sgh config add pattern "^service-" --org my-org --include

# Always skip these repos (regex) — exclude wins over include
sgh config add pattern ".*-legacy$" --org my-org --exclude
sgh config add pattern ".*-archive$" --org my-org --exclude

# Now all commands automatically respect these patterns:
sgh branch list --org my-org          # only api-* and service-* repos
sgh pr list --org my-org              # same filtering
sgh workflow list --org my-org        # same filtering
```

#### 2. CLI flags (per-command override)
Pass `-r` / `--repo` to target specific repos, or `--exclude-repo` to skip some:

```bash
# Only run on specific repos (fuzzy matched against config repository list)
sgh branch create --org my-org --new feature --ref main -r api-gateway -r service-auth

# Exclude specific repos for this run only
sgh pr list --org my-org --exclude-repo legacy-app --exclude-repo test-harness
```

> **Note:** Even with `-r`, config `exclude` patterns still apply. A repo in your  
> `exclude` list will be skipped even if explicitly named with `-r`.

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

# Reset a specific org from configuration
sgh config reset --org my-org

# Reset all configuration (skip prompt)
sgh config reset --force

# Or manually delete the config file
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
