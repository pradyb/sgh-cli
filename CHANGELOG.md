# Changelog

All notable changes to sgh-cli will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed
- **`workflow approve --watch` shows the run's progress**: each job and step is printed as it starts and finishes, interleaved with the gate prompts and decisions, so you no longer need `workflow view --watch` in a second terminal to see what the run is doing between gates. With `--json`, the NDJSON stream gains the same `job_started`, `job_completed`, `step_started`, `step_completed` events as `view --watch --json`, plus a leading `watch_started` event carrying the `run_id` being watched (useful when `--run` was omitted) — consumers that assumed only gate/`run_done` events should ignore unknown `kind`s. Costs one extra API call per poll; a failed jobs fetch never stops the watch (#67)

### Fixed
- **`workflow approve --watch` without `--run` printed `run 0`** in the "Watching …" header and the per-gate confirmation prompt; it now shows the latest waiting run that was actually resolved (#67)
- **`workflow view --watch --json` reported a job's `job_completed` before its own steps** when the whole job finished between two polls; a job's step events now always come before its completion (#67)

## [1.4.0] - 2026-09-30

### Added
- **`workflow view --watch --json`**: streams each job/step status transition as compact NDJSON (one JSON object per line: `job_started`, `job_completed`, `step_started`, `step_completed`, `run_done`) until the run completes, for scripting or logging a watched run — previously `--watch` and `--json` on `view` were mutually exclusive (#64)
- **`workflow approve --watch`**: stays attached to a run and decides each new environment approval gate as it appears (after confirmation, unless `--yes`), until the run completes — no more re-running `workflow approve` after every gate on a multi-stage deployment. Polls every `--interval` seconds (default 10, minimum 5) and can give up after `--timeout` (e.g. `30m`, default: no timeout). `--watch --yes` additionally requires `--environment`, so an unattended watch never approves a gate that didn't exist when it started. Ctrl-C stops cleanly without deciding anything further (#26)
- **Per-owner tokens are now stored in the OS keyring** (macOS Keychain, Windows Credential Manager, Linux Secret Service), not the config file. `sgh config set token --org <owner>` prompts for the value interactively (masked, or reads piped stdin for scripts/CI) instead of taking it as a command argument, so it never lands in shell history or the config file. `sgh config list` shows presence and source (`keyring`/`plaintext`) per owner, never the value; `sgh config remove token --org <owner>` removes it. A pre-existing plaintext token is migrated into the keyring automatically on first run, with a one-line notice. If no keyring is available (headless Linux, some containers), `sgh` falls back to the config file in plain text, with a warning — every command keeps working either way (#1)

### Changed
- **BREAKING: `sgh config set token <value> --org <owner>` no longer accepts the token as a positional argument.** Run `sgh config set token --org <owner>` and enter the value at the prompt (or pipe it: `echo "$TOKEN" | sgh config set token --org <owner>`)

### Fixed
- **Update notice still missed the upgrade command when GOBIN/GOPATH itself was a symlink**: the running binary's path was resolved through symlinks before comparison (needed for Homebrew's `Cellar` symlink), but the `GOBIN`/`GOPATH` candidates from #53 weren't — so on macOS, where `/tmp` is a symlink to `/private/tmp` by default, a `go install`ed binary under a `/tmp`-rooted `GOBIN` was silently treated as unknown. Candidates are now resolved the same way (#56)

## [1.3.2] - 2026-09-29

### Fixed
- **Update notice suggested a command that no longer works**: the "new version available" notice always printed `go install github.com/pradyb/sgh-cli@latest`, which has failed since v1.3.0 moved `main.go` to `cmd/sgh`. The notice now picks the upgrade command from where the binary is installed: `brew update && brew upgrade sgh` for Homebrew, `go install github.com/pradyb/sgh-cli/cmd/sgh@latest` for a `go install` binary, and only the releases link for anything else (e.g. a downloaded release binary) (#50)
- **`brew test sgh` no longer fails**: the Homebrew formula's self-test ran `sgh version` without `SGH_TOKEN`, which sgh rejects, so `brew test` failed even though the installed binary was fine. The test now supplies a dummy token
- **Update notice recognises `go install` binaries when `GOBIN`/`GOPATH` were set with `go env -w`**: the install-method detection only read process environment variables, so users who persisted their Go bin directory via `go env -w` fell back to the releases link instead of the `go install` command. It now also reads Go's env file (`$GOENV`, default `<user config dir>/go/env`) directly, with no `go` subprocess, and compares paths case-insensitively on Windows and macOS (#53)

## [1.3.1] - 2026-09-29

### Added
- **Homebrew install**: `brew install pradyb/tap/sgh` installs the prebuilt binary on macOS and Linux (see [Installation](docs/installation.md#option-1-homebrew)). Each release now also publishes `.tar.gz`/`.zip` archives, and the formula in `pradyb/homebrew-tap` is updated automatically on every non-prerelease tag

### Changed
- **Releases are built with GoReleaser**: the raw `sgh-<os>-<arch>` binaries and `checksums.txt` are still attached under the same names, so existing download URLs keep working; `sgh version` now reports the same tag, commit and build date as before

## [1.3.0] - 2026-09-29

### Added
- **Checks for a newer release at startup**: once a day, sgh checks GitHub for a newer release and prints a one-line stderr notice with the upgrade command if one exists (see [Upgrading](docs/installation.md#upgrading)). The check is cached (no added latency on ~24h of runs), bounded by a 2s timeout, and skipped automatically for machine output (`--json`/`--compact`), non-interactive/CI runs, and source builds without release version info; disable it explicitly with `--no-update-check` or `SGH_NO_UPDATE_CHECK`

### Changed
- **BREAKING: `go install github.com/pradyb/sgh-cli@latest` no longer works.** `main.go` moved to `cmd/sgh/`, so the installed binary is now named `sgh` directly (previously `sgh-cli`, requiring a manual rename or symlink). Use `go install github.com/pradyb/sgh-cli/cmd/sgh@latest` instead. `go build`/`go run` from source now need `./cmd/sgh` in place of `.` (see [Installation](docs/installation.md)); this does not affect the prebuilt binaries or `go build ./...`/`go test ./...` for the rest of the module
- **`workflow approve --environment` shorthand changed from `-E` to `-e`**: `-e` was free on the command and follows the project's own lowercase-by-default shorthand convention (`-E` didn't fit it — nothing else on `approve` claims `-e`). Scripts using `-E` (shipped in v1.2.0) must switch to `-e`; the long form `--environment` is unaffected

## [1.2.0] - 2026-09-29

### Added
- **`workflow view` supports `--json` / `--output json`**: run, jobs, approvals and any pending approval gate are printed as JSON, so `sgh workflow view -r repo --run N --json | jq .` works. `--watch` is rejected together with `--json` (it's an interactive view); the "Using latest workflow run" notice now goes to stderr in every mode
- **`workflow view` shows the pending approval gate**: for a run waiting on an environment gate, a "Pending approval" section lists each blocking environment, its required reviewers (users or teams), whether you can approve it, and the `workflow approve` command to run. Only fetched for `waiting` runs, so other runs cost no extra API call, and it also refreshes under `--watch`
- **`workflow view` shows approval decisions**: for runs that went through environment approval gates, an "Approvals" section lists each gate's decision (approved/rejected), the reviewer and their comment, in chronological order
- **`workflow approve`**: approve (or `--reject`) the environment approval gates a workflow run is waiting on, without opening the Actions UI. Defaults to the latest waiting run, supports `--environment`, `--comment`, several `-r` repos, `--dry-run` and `--json`, and asks for confirmation unless `--yes` is given. Gates you are not a required reviewer of are skipped and reported
- **`pr list` shows a Created column**: pull request listings now include the creation date alongside the existing columns
- **`issue list` resolves author display names and shows a linked-PR indicator**: issue authors are now shown by display name (batched via a single GraphQL lookup, chunked to stay under GitHub's 100-ID node limit) instead of just their login, and each issue shows how many pull requests are linked to close it

### Changed
- **Third-party attributions moved out of `LICENSE`**: the appended dependency notices made GitHub classify the repo as `NOASSERTION` ("Other") instead of MIT, which also trips automated license scanners. `LICENSE` is now the unmodified MIT text and the attributions live in `THIRD_PARTY_NOTICES.md`, now generated from `go list -deps` so it covers every linked module rather than just two

### Fixed
- **`workflow dispatch` silently ignored malformed `--input` values**: an input without `=` (such as a typo like `--input target` or `--input env:prod`) was dropped, so the workflow ran with its default instead of the intended value. It now fails before dispatching anything, naming the bad value, and also rejects an empty key
- **`--output json` could not be piped to `jq`**: the trailing `API calls: N` summary was printed to stdout, corrupting structured output. It now goes to stderr, and is omitted entirely for `--json`/`--compact`/`--output json|compact` so `2>&1 | jq` also works. Anything that stripped that line from stdout (e.g. with `grep`) should read stderr instead
- **Config rejected GitHub Enterprise Server usernames with underscores**: username validation enforced github.com's public rules only, so a SAML/LDAP-synced name like `jane-doe_acme` in `pull_request_assignees` or the `protected_branch` user lists failed config loading and blocked every command

## [1.1.0] - 2026-08-21

First public release.

### Fixed
- **Branch creation from a commit reported success as failure**: `branch create --from-commit` passed the new SHA into the error field, so every successful creation was rendered as an error
- **Retried API errors lost their guidance**: once retries were exhausted the underlying GitHub error was wrapped, so retried 401/403/404/5xx responses fell back to a generic message instead of "check your SGH_TOKEN", "check your token permissions", and so on
- **Data race in bulk operations**: result and error handlers ran on separate goroutines while sharing unsynchronised state, which could corrupt results when a success and a failure completed together
- **Proxy environment variables were ignored**: `HTTP_PROXY`, `HTTPS_PROXY`, and `NO_PROXY` are now honoured
- **Module path**: corrected to `github.com/pradyb/sgh-cli`

### Changed
- README split into a `docs/` guide (installation, authentication, configuration, commands, examples, advanced usage, troubleshooting, development)
- Added `SECURITY.md`, a disclaimer, and a security section to the README

### Internal
- Test coverage raised to 94%, enforced by an 85% floor in CI via `scripts/check-coverage.sh`
- Pre-commit hooks blocking oversized files, credential-shaped strings, and unformatted Go; `gofmt` gate added to CI
- Test-only helpers moved out of the production import graph so `testing` and `net/http/httptest` are no longer linked into the binary

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

[Unreleased]: https://github.com/pradyb/sgh-cli/compare/v1.4.0...HEAD
[1.4.0]: https://github.com/pradyb/sgh-cli/compare/v1.3.2...v1.4.0
[1.3.2]: https://github.com/pradyb/sgh-cli/compare/v1.3.1...v1.3.2
[1.3.1]: https://github.com/pradyb/sgh-cli/compare/v1.3.0...v1.3.1
[1.3.0]: https://github.com/pradyb/sgh-cli/compare/v1.2.0...v1.3.0
[1.2.0]: https://github.com/pradyb/sgh-cli/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/pradyb/sgh-cli/compare/v1.0.0...v1.1.0
[1.0.0]: https://github.com/pradyb/sgh-cli/releases/tag/v1.0.0
