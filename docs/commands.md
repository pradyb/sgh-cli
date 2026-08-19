# Command reference

Every command follows the same shape:

```
sgh <command> <subcommand> [flags]
```

Run `sgh <command> --help` for the authoritative flag list of any command — this page is a map, not a replacement for it.

- [Command groups](#command-groups)
- [Command details](#command-details)
- [Shortcuts](#shortcuts)
- [Global flags](#global-flags)
- [Flag shorthand convention](#flag-shorthand-convention)
- [Per-command flags](#per-command-flags)

## Command groups

### Repository management

| Command | Alias | Subcommands | Description |
|---|---|---|---|
| `repo` | — | `list`, `search`, `archive`, `visibility` | List, search, archive, and manage repository visibility |
| `clone` | — | — | Clone many repositories at once |
| `commit` | `ci` | `list` | List recent commits across repositories |
| `issue` | `is` | `list`, `view`, `create` | List, view, and create issues |

### Git operations

| Command | Alias | Subcommands | Description |
|---|---|---|---|
| `branch` | `br` | `list`, `create`, `rename`, `delete` | Branch management across repositories |
| `tag` | `tg` | `list`, `create`, `delete` | Tag management across repositories |
| `pr` | — | `create`, `list`, `view`, `review`, `update`, `merge`, `close`, `reopen` | Full pull request lifecycle |
| `protected-branch` | `pb` | `list`, `update`, `delete` | Branch protection rules |

### CI/CD and release

| Command | Alias | Subcommands | Description |
|---|---|---|---|
| `workflow` | `wf` | `list`, `view`, `rerun`, `cancel`, `dispatch` | GitHub Actions workflow runs |
| `post-release` | — | — | Hotfix branch and/or release tag across repositories |

### Organization

| Command | Alias | Subcommands | Description |
|---|---|---|---|
| `org` | — | `list` | Organizations the token belongs to |
| `team` | — | `list` | Teams and their members |
| `security` | `sec` | `list`, `view`, `update` | Secret scanning alerts |
| `audit` | `al` | `list` | Organization audit log |

### Utilities

| Command | Alias | Subcommands | Description |
|---|---|---|---|
| `config` | `cfg` | `list`, `validate`, `add`, `set`, `remove`, `reset` | Manage and validate configuration |
| `tui` | — | — | Full-screen interactive dashboard |
| `whoami` | `me` | — | Authenticated user's profile |
| `health` | — | — | Check connectivity and token health |
| `shortcuts` | — | — | List available command shortcuts |
| `version` | — | — | Version, commit SHA, build date |
| `completion` | — | `bash`, `zsh`, `fish`, `powershell` | Generate shell completion scripts |

## Command details

### Repository management

- `repo list --org <org> [--all]` — list repositories
- `repo search --org <org> --query <text> [--language <lang>] [--topic <topic>]` — search repositories
- `repo archive --org <org> [-r <repo>...] [--unarchive]` — archive or unarchive in bulk
- `repo visibility --org <org> [-r <repo>...] --visibility public|private` — set visibility
- `clone --org <org> [--branch <branch>]` — clone repositories
- `commit list --org <org> [--days <n>] [--since <date>] [--until <date>]` — list commits

### Branches and tags

- `branch list --org <org> [--filter <regex>]` — list branches
- `branch create --org <org> --new <name> --ref <ref>` — create branches
- `branch rename --org <org> --old <name> --new <name>` — rename a branch
- `branch delete --org <org> --branch <name>` — delete branches
- `tag list --org <org> [--filter <regex>]` — list tags
- `tag create --org <org> --tag <name> --head <ref> --message <msg>` — create tags
- `tag delete --org <org> --tag <name>` — delete tags

### Issues

- `issue list --org <org> [--state open|closed|all] [-A <author>] [-a <assignee>] [-l <label>]` — list and filter issues
- `issue view --org <org> -r <repo> --issue <num>` — view issue details
- `issue create --org <org> -r <repo> --title <title> [--body <body>] [--assignee <user>] [--label <label>]` — create an issue

### Pull requests

- `pr create --org <org> --title <title> --head <branch> --base <branch> [-r <repo>...] [--label <name>]` — create PRs
- `pr list --org <org> [--state open|closed|merged|all] [--sort repo|title|author|status]` — list PRs (open only by default)
- `pr view --org <org> -r <repo> --pr <num>` — view details, files, checks, reviews
- `pr review --org <org> -r <repo> --pr <num> --approve|--comment|--request-changes` — review a PR
- `pr update --org <org> -r <repo> --pr <num> --state open|closed` — update PR state
- `pr merge --org <org> -r <repo> --pr <num>` — merge a PR
- `pr close` / `pr reopen --org <org> -r <repo> --pr <num>` — close or reopen
- `pr list --interactive` — interactive PR management

### Protected branches

- `protected-branch list --org <org> [--branch <branch>]` — list protection settings
- `protected-branch update --org <org> --branch <branch> [--lock] [--remove-status-checks]` — update rules
- `protected-branch delete --org <org> --branch <branch>` — remove protection

### Workflow runs

- `workflow list --org <org> [--running|--queued|--failed] [--branch <name>] [--workflow <name>]` — list runs
- `workflow view --org <org> -r <repo> --run <id> [--watch] [--interval <sec>]` — view or live-monitor a run
- `workflow rerun --org <org> -r <repo> --run <id>` — re-trigger a run
- `workflow cancel --org <org> -r <repo> --run <id>` — cancel an in-progress run
- `workflow dispatch --org <org> --workflow <file> --ref <branch> [--input key=value]` — trigger a `workflow_dispatch` event

### Security and audit

- `security list --org <org> [--state open|resolved] [--secret-type <type>]` — list secret scanning alerts
- `security view --org <org> -r <repo> --alert <num>` — view alert details
- `security update --org <org> -r <repo> --alert <num> --state resolved --resolution <reason> [--comment <text>]` — resolve or reopen an alert
- `audit list --org <org> [-c <count>] [-i web|git|all] [-p <phrase>]` — read the organization audit log

### Organization and configuration

- `org list` — organizations the token belongs to (no `--org` needed)
- `team list --org <org> [--team <name>] [--all]` — teams and members
- `config list` (alias `show`) — current configuration, token status, owner type
- `config validate` — check config for errors
- `config add <key> <value>` — add org, repo, pattern, or PR assignee
- `config set <key> <value>` — set token, tagger identity, or owner type
- `config remove <key> <value>` (aliases `rm`, `delete`) — remove an org, repo, pattern, or PR assignee
- `config reset [--org <name>] [--yes]` — remove one or all organizations

## Shortcuts

Single-word shortcuts exist for the most common `list` and `view` subcommands. Run `sgh shortcuts` to print the current list.

| Shortcut | Expands to | Shortcut | Expands to |
|---|---|---|---|
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
| `wai` | `whoami` | | |

Shortcuts accept exactly the same flags as the command they expand to:

```bash
sgh orl                                    # sgh org list
sgh prl --org my-org -A john-doe           # sgh pr list --org my-org --author john-doe
sgh wfl --org my-org --running             # sgh workflow list --org my-org --running
sgh brl --org my-org --filter "Release-"   # sgh branch list --org my-org --filter "Release-"
```

## Global flags

Available on every command:

| Flag | Description |
|---|---|
| `-h, --help` | Show help |
| `-o, --org <name>` | Organization or user (env `SGH_ORG`; required by most commands) |
| `-v, --verbose` | Verbose output |
| `-L, --log-response` | Log HTTP responses for debugging |
| `-w, --workers <n>` | Concurrent workers (default 5, env `SGH_WORKERS`) |
| `-O, --output <mode>` | `table` (default), `compact`, or `json` |
| `-C, --compact` | Shorthand for `--output compact` |
| `-J, --json` | Shorthand for `--output json` |
| `--dry-run` | Preview changes without executing them |
| `--no-color` | Disable colored output (env `NO_COLOR`) |
| `--limit <n>` | Cap the final combined output across all repos (0 = no cap) |

> **`--limit` and `--last` are different controls.** `--last` (on `pr list`, `issue list`, `workflow list`) caps how many items are fetched **per repository** from the API. `--limit` truncates the **final combined output** across all repositories. They compose: `--last 20 --limit 50`.

## Flag shorthand convention

One rule governs every single-letter shorthand in the CLI:

> Use lowercase by default. Use uppercase only when the lowercase letter is already taken — by a global persistent flag, or by another flag on the same command.

**Reserved globally** — never reused by any command:

| `-o` | `-v` | `-w` | `-L` | `-O` | `-C` | `-J` |
|---|---|---|---|---|---|---|
| `--org` | `--verbose` | `--workers` | `--log-response` | `--output` | `--compact` | `--json` |

**Uppercase in use**, and what took the lowercase:

| Shorthand | Flag | Command | Lowercase taken by |
|---|---|---|---|
| `-B` | `--base` | `pr create/list` | `-b` = `--body` |
| `-H` | `--head` | `pr create/list`, `tag create` | `-h` = help (reserved by Cobra) |
| `-A` | `--author` | `pr list`, `issue list` | `-a` = `--assignee` |
| `-R` | `--reviewer` | `pr list` | `-r` = `--repository` |
| `-R` | `--run` | `workflow view/rerun/cancel` | `-r` = `--repository` |
| `-R` | `--ref` | `post-release` | `-r` = `--repository` |
| `-W` | `--watch` | `workflow view` | `-w` = global `--workers` |
| `-W` | `--workflow` | `workflow dispatch` | `-w` = global `--workers` |
| `-V` | `--visibility` | `repo visibility` | `-v` = global `--verbose` |

## Per-command flags

| Flag | Available on |
|---|---|
| `--sort <field>` | `pr`, `branch`, `tag`, `issue`, `security`, `workflow` list commands |
| `--last <count>` | `pr list`, `issue list`, `workflow list` — items fetched per repository |
| `--state <value>` | `pr list`, `issue list` — `open`, `closed`, `merged`, `all` |
| `--filter <regex>` | `branch list`, `tag list` |
| `--all` | `team list` |
| `-l, --label <name>` | `pr list`, `pr create`, `issue list`, `issue create` |
| `-A, --author <login>` | `pr list`, `issue list` |
| `-a, --assignee <login>` | `pr list`, `issue list`, `issue create` |
| `--since <YYYY-MM-DD>` | `pr list`, `commit list` |
| `--until <YYYY-MM-DD>` | `commit list` |
| `--running`, `--queued`, `--failed` | `workflow list` |
| `--branch <name>` | `workflow list` |
| `--workflow <name>` | `workflow list` (partial match) |
| `--watch`, `--interval` | `workflow view` |
| `--input key=value` | `workflow dispatch` (repeatable) |
| `--short` | `version` |
| `--json` | `health` |

> On `workflow list`, `--last` has the shorthand `-l`. On `pr list` and `issue list`, `-l` is taken by `--label`.

---

**Next:** [Usage examples](examples.md) · [Advanced usage](advanced.md)
