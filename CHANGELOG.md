# Changelog

All notable changes to sgh-cli will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.0.0] - 2025-03-10

### Added
- **TUI Dashboard**: Interactive terminal UI for managing repositories, PRs, workflows, and issues
- **Audit Log**: View repository audit logs with filtering and export options
- **Diff Preview**: Inline diff preview for pull requests in the TUI
- **PR Actions**: Approve, merge, and close pull requests directly from the TUI
- **Bulk Branch Operations**: Create, delete, and rename branches across multiple repositories
- **Bulk Tag Operations**: Create and delete tags across multiple repositories
- **Pull Request Automation**: Create, list, update, and merge PRs in bulk
- **GitHub Actions / Workflow Runs**: List, view, rerun, and cancel workflow runs with live monitoring
- **Protected Branch Management**: Configure and update branch protection rules
- **Post-Release Workflows**: Automate post-release merging and tagging
- **Team Management**: List teams and members across your organisation
- **Repository Cloning**: Clone multiple repositories concurrently
- **Commit Tracking**: Track and compare commits across repositories
- **Issue Management**: List and filter issues across repositories
- **Security Alerts**: View and manage secret scanning alerts
- **Health Check**: Validate configuration and GitHub connectivity
- **Shell Completion**: Built-in completion for Bash, Zsh, Fish, and PowerShell
- **Command Shortcuts**: Short aliases for frequently used commands
- **Flexible Output Modes**: `--output table|compact|json` with `--compact` and `--json` shorthands
- **Adaptive Terminal Tables**: Auto-resize to terminal width with coloured status indicators
- **Concurrent Processing**: Configurable worker threads (`--workers`) for fast bulk operations
- **Rate Limit Management**: Built-in rate limiting and automatic retry with exponential backoff
- **Graceful Shutdown**: Signal handling for clean interruption of long-running operations
- **Verbose Logging**: `--verbose` flag for detailed debug output via zerolog
- **Include/Exclude Patterns**: Regex-based repository filtering with `--include` and `--exclude`
- **NO_COLOR support**: Respects the `NO_COLOR` environment variable
- **Global org/worker env vars**: `SGH_ORG` and `SGH_WORKERS` to avoid repeating flags

[Unreleased]: https://github.com/prady-lab/sgh-cli/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/prady-lab/sgh-cli/releases/tag/v1.0.0
