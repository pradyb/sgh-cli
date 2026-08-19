# Usage examples

Every example assumes `SGH_TOKEN` is set. Set `SGH_ORG` too and you can drop `--org` from all of them.

> **Preview first.** Anything that writes accepts `--dry-run`. On a command that touches every repository in an organization, running it once with `--dry-run` is the difference between a bulk operation and a bulk incident.

- [Branches](#branches) · [Tags](#tags) · [Issues](#issues) · [Pull requests](#pull-requests)
- [Protected branches](#protected-branches) · [Post-release](#post-release) · [Repositories](#repositories)
- [Workflow runs](#workflow-runs) · [Security alerts](#security-alerts) · [Audit log](#audit-log)
- [Organizations and teams](#organizations-and-teams) · [Identity](#identity) · [Interactive dashboard](#interactive-dashboard)

## Branches

```bash
# List branches across all repos
sgh branch list --org my-org

# Filter by regex, optionally scoped to one repo
sgh branch list --org my-org --filter "Release-"
sgh branch list --org my-org --filter "feature/" -r my-app

# Cut a release branch everywhere
sgh branch create --org my-org --new Release-1.1 --ref Release-1.0

# Hotfix branch on selected repos only
sgh branch create --org my-org --new hotfix --ref main -r critical-app -r important-service

# Rename master to main across the org
sgh branch rename --org my-org --old master --new main

# Delete a stale branch
sgh branch delete --org my-org --branch old-feature -r legacy-app
```

## Tags

```bash
sgh tag list --org my-org
sgh tag list --org my-org --filter "^v1\."
sgh tag list --org my-org -r app1 -r app2

# Create release tags across repositories
sgh tag create --org my-org --tag v1.0.0 --head Release-1.0 --message 'Release v1.0.0'
sgh tag create --org my-org --tag v2.1.0 --head main --message 'Version 2.1.0' -r app1 -r app2

sgh tag delete --org my-org --tag old-version -r legacy-app
```

`tag create` uses the tagger identity from your config — see [`config set tagger-name`](configuration.md#managing-configuration).

## Issues

```bash
sgh issue list --org my-org
sgh issue list --org my-org -r my-app -r other-service

# Filter by author, assignee, label, state
sgh issue list --org my-org -A jane-doe
sgh issue list --org my-org -a john-doe --label bug --state all

sgh issue view --org my-org -r my-app --issue 42

sgh issue create --org my-org -r my-app \
  --title "Bug: login fails" \
  --body "Steps to reproduce..." \
  --assignee john-doe --label bug,high-priority
```

## Pull requests

```bash
# Open the same PR across many repositories
sgh pr create --org my-org --title "Security Update" \
  --body "Update dependencies" --head security-patch --base main

# List — open only by default
sgh pr list --org my-org -r app1 -r app2 --base main
sgh pr list --org my-org -r app1 -r app2 --state all
sgh pr list --org my-org -A john-doe -a jane-doe --last 10
sgh pr list --org my-org --label bug --since 2024-01-01

# Details: files, checks, reviews
sgh pr view --org my-org -r my-app --pr 42

# Review
sgh pr review --org my-org -r my-app --pr 42 --approve
sgh pr review --org my-org -r my-app --pr 42 --request-changes --body "Please fix the tests"
sgh pr review --org my-org -r my-app --pr 42 --comment --body "Looks good overall"

# State changes
sgh pr update --org my-org -r my-app --pr 123 --state closed
sgh pr close  --org my-org -r my-app --pr 123
sgh pr reopen --org my-org -r my-app --pr 123

# Interactive selection UI
sgh pr list --org my-org --interactive
```

## Protected branches

```bash
sgh protected-branch list --org my-org
sgh protected-branch list --org my-org --branch main

# Grant bypass and push access
sgh protected-branch update --org my-org --branch main -r my-app \
  --add-bypass-user admin --add-push-user ci-bot

# Lock the branch and drop all required status checks
sgh protected-branch update --org my-org --branch main -r my-app \
  --lock --remove-status-checks
```

## Post-release

```bash
# Merge back and tag in one pass
sgh post-release --org my-org --base main --head Release-1.0 \
  --create-tag --title "Release 1.0"

# Selected repositories only
sgh post-release --org my-org --base develop --head feature-complete \
  -r service1 -r service2

# Everything except these
sgh post-release --org my-org --base main --head Release-1.0 \
  -e legacy-app -e deprecated-service
```

## Repositories

```bash
sgh repo list --org my-org
sgh repo search --org my-org --query "api"
sgh repo search --org my-org --language go --topic microservice

# Archive and unarchive in bulk
sgh repo archive --org my-org -r old-service -r legacy-api
sgh repo archive --org my-org -r old-service --unarchive

# Visibility
sgh repo visibility --org my-org -r my-repo --visibility private
sgh repo visibility --org my-org -r internal-tool --visibility public

# Clone everything on a given branch
sgh clone --org my-org --branch develop

# Commit activity
sgh commit list --org my-org --days 7 --details --include-merge-commits
sgh commit list --org my-org --since 2026-08-01 --until 2026-08-15
```

## Workflow runs

```bash
sgh workflow list --org my-org
sgh workflow list --org my-org --running
sgh workflow list --org my-org --failed --branch main
sgh workflow list --org my-org --workflow "CI Build"
sgh workflow list --org my-org --sort status

# Details, and live monitoring
sgh workflow view --org my-org -r my-app --run 123456789
sgh workflow view --org my-org -r my-app --run 123456789 --watch
sgh workflow view --org my-org -r my-app --run 123456789 --watch --interval 5

sgh workflow rerun  --org my-org -r my-app --run 123456789
sgh workflow cancel --org my-org -r my-app --run 123456789

# Fan a workflow_dispatch out across repositories
sgh workflow dispatch --org my-org --workflow deploy.yml --ref main
sgh workflow dispatch --org my-org -r app1 -r app2 \
  --workflow release.yml --ref main \
  --input env=production --input dry_run=false
```

## Security alerts

```bash
sgh security list --org my-org
sgh security list --org my-org --state open
sgh security list --org my-org --state resolved
sgh security list --org my-org --secret-type aws_access_key
sgh security list --org my-org -r api-service -r web-app

sgh security view --org my-org -r my-app --alert 1
sgh security view --org my-org -r my-app --alert 5 --json

# Resolve, with a reason
sgh security update --org my-org -r my-app --alert 1 \
  --state resolved --resolution false_positive
sgh security update --org my-org -r my-app --alert 2 \
  --state resolved --resolution revoked --comment "Key has been rotated"
sgh security update --org my-org -r my-app --alert 4 \
  --state resolved --resolution used_in_tests

# Reopen
sgh security update --org my-org -r my-app --alert 3 --state open

# Preview without applying
sgh security update --org my-org -r my-app --alert 1 \
  --state resolved --resolution false_positive --dry-run
```

## Audit log

Requires an organization and a token with **Organization Administration: Read**.

```bash
sgh audit list --org my-org

# Last 100 entries
sgh audit list --org my-org -c 100

# Only web-UI events, or only git events
sgh audit list --org my-org -i web
sgh audit list --org my-org -i git

# Filter by action phrase
sgh audit list --org my-org -p repo.create
```

## Organizations and teams

```bash
# No --org needed — lists everything the token can see
sgh org list
sgh orl -J
sgh orl --limit 5

sgh team list --org my-org
sgh team list --org my-org --team developers --all
sgh team list --org my-org --members 100
```

## Identity

```bash
sgh whoami
sgh me

# Full profile as JSON
sgh whoami --json
sgh wai -J
```

## Interactive dashboard

```bash
sgh tui
```

A full-screen terminal dashboard for browsing repositories, PRs, issues, branches, tags, workflows, and teams without retyping commands.

---

**Next:** [Advanced usage](advanced.md) · [Troubleshooting](troubleshooting.md)
