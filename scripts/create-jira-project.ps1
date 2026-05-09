# sgh-cli Jira Project Setup Script (Company-Managed)
# Creates: Project SGHV2 (company-managed Scrum), Epic, User Stories, Sub-tasks and marks all Done

$JIRA_URL   = $env:JIRA_URL   ?? "https://your-domain.atlassian.net"
$EMAIL      = $env:JIRA_EMAIL ?? "your-email@example.com"
$TOKEN      = $env:JIRA_TOKEN ?? "your-jira-api-token"
$ACCOUNT_ID = $env:JIRA_ACCOUNT_ID ?? "your-account-id"
$PROJECT_KEY_TARGET = "SGHV2"

$AUTH = [Convert]::ToBase64String([Text.Encoding]::ASCII.GetBytes("${EMAIL}:${TOKEN}"))
$HEADERS = @{
    "Authorization" = "Basic $AUTH"
    "Content-Type"  = "application/json"
    "Accept"        = "application/json"
}

function Invoke-Jira {
    param($method, $path, $body = $null)
    $uri = "$JIRA_URL$path"
    $params = @{ Uri = $uri; Method = $method; Headers = $HEADERS; UseBasicParsing = $true }
    if ($body) { $params["Body"] = ($body | ConvertTo-Json -Depth 10 -Compress) }
    try {
        $resp = Invoke-WebRequest @params
        return ($resp.Content | ConvertFrom-Json)
    } catch {
        Write-Host "  ERROR [$method $path]: $($_.ErrorDetails.Message)" -ForegroundColor Red
        return $null
    }
}

function Get-DoneTransitionId {
    param($issueKey)
    $r = Invoke-Jira "GET" "/rest/api/3/issue/$issueKey/transitions"
    if (-not $r) { return $null }
    $t = $r.transitions | Where-Object { $_.to.name -in @("Done","Closed","Resolved") } | Select-Object -First 1
    if (-not $t) { $t = $r.transitions | Where-Object { $_.name -in @("Done","Close","Resolve","Mark as Done") } | Select-Object -First 1 }
    if ($t) { return $t.id }
    return $null
}

# STEP 1: Create Company-Managed Project SGHV2
Write-Host "[1/5] Setting up Jira Project $PROJECT_KEY_TARGET (company-managed)" -ForegroundColor Cyan
$projects = Invoke-Jira "GET" "/rest/api/3/project"
$existing = $projects | Where-Object { $_.key -eq $PROJECT_KEY_TARGET } | Select-Object -First 1

if ($existing) {
    $PROJECT_KEY = $existing.key
    $PROJECT_ID  = $existing.id
    Write-Host "  Reusing existing project $PROJECT_KEY (id=$PROJECT_ID)" -ForegroundColor Yellow
} else {
    $pb = @{
        name               = "sgh-cli (Company)"
        key                = $PROJECT_KEY_TARGET
        projectTypeKey     = "software"
        projectTemplateKey = "com.pyxis.greenhopper.jira:gh-scrum-template"
        description        = "Simple GitHub CLI - Bulk GitHub organization management tool built in Go (company-managed)"
        leadAccountId      = $ACCOUNT_ID
        assigneeType       = "PROJECT_LEAD"
    }
    $proj = Invoke-Jira "POST" "/rest/api/3/project" $pb
    if (-not $proj) { Write-Host "Failed to create project"; exit 1 }
    $PROJECT_KEY = $proj.key
    $PROJECT_ID  = $proj.id
    Write-Host "  Created company-managed project $PROJECT_KEY (id=$PROJECT_ID)" -ForegroundColor Green
}

# STEP 2: Get Issue Types
Write-Host "[2/5] Fetching issue types" -ForegroundColor Cyan
$its = Invoke-Jira "GET" "/rest/api/3/issuetype/project?projectId=$PROJECT_ID"
$epicType    = $its | Where-Object { $_.name -eq "Epic"    } | Select-Object -First 1
$storyType   = $its | Where-Object { $_.name -eq "Story"   } | Select-Object -First 1
$subtaskType = $its | Where-Object { $_.name -in @("Sub-task","Subtask") -and $_.subtask -eq $true } | Select-Object -First 1
Write-Host "  Epic=$($epicType.name)($($epicType.id)) Story=$($storyType.name)($($storyType.id)) Sub-task=$($subtaskType.name)($($subtaskType.id))"

# STEP 3: Create Epic
Write-Host "[3/5] Creating Epic" -ForegroundColor Cyan
$ef = @{
    project   = @{ key = $PROJECT_KEY }
    issuetype = @{ id  = $epicType.id }
    summary   = "sgh-cli: Simple GitHub CLI for Bulk Organization Management"
    description = @{
        type = "doc"; version = 1
        content = @( @{ type = "paragraph"; content = @( @{ type = "text"; text = "Build a powerful Go-based CLI tool that enables bulk management of GitHub repositories, branches, tags, pull requests, workflows, and more across an entire GitHub organization. Implemented with concurrent processing, interactive TUI, rate limiting, circuit breaker, and flexible output modes." } ) } )
    }
}
$epic = Invoke-Jira "POST" "/rest/api/3/issue" @{ fields = $ef }
if (-not $epic) { Write-Host "Failed to create Epic"; exit 1 }
$EPIC_KEY = $epic.key
Write-Host "  Created Epic: $EPIC_KEY" -ForegroundColor Green

# STEP 4: User Stories and Tasks data
$stories = @(
    @{
        s = "Core Infrastructure and Configuration"
        d = "As a developer, I want a solid foundation with HTTP client, rate limiting, retry logic, circuit breaker, authentication, config file management, env vars, and graceful shutdown so that all CLI commands are reliable and configurable."
        t = @(
            "Implement HTTP client with SGH_TOKEN authentication support",
            "Implement config file loading for Windows and Linux/Mac platforms",
            "Add environment variable support: SGH_TOKEN SGH_ORG SGH_WORKERS NO_COLOR",
            "Implement rate limit detection and automatic backoff handler",
            "Implement exponential backoff retry mechanism for transient failures",
            "Implement circuit breaker pattern for failing API endpoints",
            "Add graceful shutdown with SIGINT and SIGTERM signal handling",
            "Implement verbose logging with zerolog via --verbose flag",
            "Add GitHub token format validation and test token rejection",
            "Implement context propagation across all CLI commands"
        )
    },
    @{
        s = "Repository Management"
        d = "As a GitHub admin, I want to list, search, archive/unarchive, and toggle visibility of repositories so that I can manage the full repository lifecycle in bulk."
        t = @(
            "Implement sgh repo list command with org-level repository listing",
            "Implement sgh repo search command with keyword and filter support",
            "Implement sgh repo archive command for bulk archive and unarchive",
            "Implement sgh repo visibility command for bulk public/private toggle",
            "Add --include and --exclude regex pattern flags for repository filtering",
            "Add --limit flag to cap number of results returned",
            "Support JSON table and compact output modes for all repo commands",
            "Add rpl rps rpa rpv shortcut aliases for repo subcommands"
        )
    },
    @{
        s = "Branch Management"
        d = "As a developer, I want to list, create, rename, and delete branches across multiple repositories so that I can manage branch operations at scale."
        t = @(
            "Implement sgh branch list with filter and sort-by support",
            "Implement sgh branch create with --new and --ref flags",
            "Implement sgh branch rename with --old and --new flags",
            "Implement sgh branch delete with safety confirmation checks",
            "Add --repository flag to target specific repositories",
            "Add --exclude flag to skip specific repositories",
            "Support dry-run mode for all branch mutating operations",
            "Add brl brc brr brd shortcut aliases"
        )
    },
    @{
        s = "Tag Management"
        d = "As a release manager, I want to create and delete tags across multiple repositories so that I can manage releases consistently across the organization."
        t = @(
            "Implement sgh tag list command with filter support",
            "Implement sgh tag create with --tag and --ref flags",
            "Implement sgh tag delete for bulk tag removal across repos",
            "Support concurrent tag operations with configurable worker count",
            "Add tgl tgc tgd shortcut aliases"
        )
    },
    @{
        s = "Pull Request Automation"
        d = "As a developer, I want to create, list, view, review, update, merge, close, and reopen pull requests in bulk so that I can automate PR workflows at scale."
        t = @(
            "Implement sgh pr create with title body head base and label flags",
            "Implement sgh pr list with status and branch filters",
            "Implement sgh pr view command for detailed PR information",
            "Implement sgh pr review for approve request-changes and comment actions",
            "Implement sgh pr update for updating title body and labels",
            "Implement sgh pr merge with merge squash and rebase strategy options",
            "Implement sgh pr close for bulk pull request closure",
            "Implement sgh pr reopen for re-opening closed pull requests",
            "Implement interactive PR selection TUI using bubbletea list component",
            "Add prl prv prc prx shortcut aliases"
        )
    },
    @{
        s = "GitHub Actions Workflow Management"
        d = "As a DevOps engineer, I want to list, view, rerun, cancel, and dispatch workflow runs with live monitoring so that I can manage CI/CD pipelines at scale."
        t = @(
            "Implement sgh workflow list with --running --queued --failed status filters",
            "Implement sgh workflow view with job and step-level details",
            "Implement --watch flag for live polling until run completion",
            "Implement --interval flag to configure polling interval in seconds",
            "Implement sgh workflow rerun for re-triggering failed workflow runs",
            "Implement sgh workflow cancel for stopping in-progress workflow runs",
            "Implement sgh workflow dispatch for workflow_dispatch events across repos",
            "Add wfl wfv wfr wfc wfd shortcut aliases"
        )
    },
    @{
        s = "Protected Branch Management"
        d = "As a GitHub admin, I want to configure and update branch protection rules across repositories so that I can enforce code quality standards organization-wide."
        t = @(
            "Implement sgh protectedbranch list to view current protection rules",
            "Implement sgh protectedbranch update to configure protection settings",
            "Support required status checks PR reviews and admin enforcement flags",
            "Add pbl and pbu shortcut aliases"
        )
    },
    @{
        s = "Issue Management"
        d = "As a project manager, I want to create and list GitHub issues across repositories so that I can track work items at the organization level."
        t = @(
            "Implement sgh issue list with label state and assignee filters",
            "Implement sgh issue view for detailed issue information",
            "Implement sgh issue create with title body label and assignee flags",
            "Support cross-repo issue listing with org-level aggregation",
            "Add isl isv isc shortcut aliases"
        )
    },
    @{
        s = "Security Alerts Management"
        d = "As a security engineer, I want to view and manage secret scanning alerts so that I can maintain security compliance organization-wide."
        t = @(
            "Implement sgh security list for secret scanning alerts",
            "Support filtering by alert state open resolved dismissed",
            "Add repository and org-level alert aggregation",
            "Add scl shortcut alias"
        )
    },
    @{
        s = "Audit Log"
        d = "As a compliance officer, I want to view, filter, and export repository audit logs so that I can maintain audit trails."
        t = @(
            "Implement sgh audit command with org-level audit log fetch via GitHub API",
            "Support filtering by action type actor and date range",
            "Support JSON export of audit log entries",
            "Implement audit log display in TUI detail panel"
        )
    },
    @{
        s = "Team Management"
        d = "As an org admin, I want to list teams and their members so that I can manage access and permissions efficiently."
        t = @(
            "Implement sgh team list for org-level team listing",
            "Implement team member listing per team",
            "Support output in table compact and JSON modes",
            "Add tml shortcut alias"
        )
    },
    @{
        s = "Post-Release Workflow Automation"
        d = "As a release engineer, I want to automate post-release activities like merging release branches and creating tags so that releases are consistent."
        t = @(
            "Implement sgh postrelease command for post-release automation",
            "Support auto-merge of release branches to main or develop",
            "Support auto-tagging after successful release merges",
            "Add dry-run mode for post-release workflow preview"
        )
    },
    @{
        s = "Clone and Commit Tracking"
        d = "As a developer, I want to clone multiple repositories concurrently and track commits across repos so that I can manage large codebases efficiently."
        t = @(
            "Implement sgh clone for concurrent multi-repo cloning",
            "Implement sgh commit list for commit tracking across repositories",
            "Support --since and --until date flags for commit filtering",
            "Implement concurrent cloning with configurable --workers count"
        )
    },
    @{
        s = "Interactive TUI Dashboard"
        d = "As a developer, I want an interactive terminal UI dashboard to browse repositories, PRs, issues, and workflows with keyboard navigation so that I can manage GitHub without leaving the terminal."
        t = @(
            "Implement repo selector sidebar panel using bubbletea",
            "Implement command menu panel with categorized GitHub operations",
            "Implement content panel with sortable and filterable data tables",
            "Implement detail panel for PR issue workflow and audit details",
            "Implement inline diff preview for pull requests in TUI",
            "Add PR actions in TUI: approve merge and close",
            "Implement status bar with org name repo count and key hints",
            "Add keyboard navigation: ? q / arrow keys enter",
            "Implement TUI caching to reduce redundant API calls",
            "Support NO_COLOR environment variable for colorless rendering"
        )
    },
    @{
        s = "Developer Experience and CLI Quality"
        d = "As a user, I want shell completion, shortcuts, dry-run mode, flexible output, and helpful error messages so that the CLI is pleasant and productive."
        t = @(
            "Implement shell completion for Bash Zsh Fish and PowerShell",
            "Implement sgh shortcuts command listing all short aliases with expansions",
            "Implement --dry-run flag with preview output for all mutating commands",
            "Implement --output table|compact|json flag across all commands",
            "Implement --compact and --json shorthand flags",
            "Implement adaptive table auto-sizing to fit terminal width",
            "Add colored status indicators in table output with NO_COLOR support",
            "Implement sgh whoami command for token identity and user info",
            "Implement sgh health command to validate config and GitHub connectivity",
            "Implement sgh version command with build version info",
            "Implement sgh config get set and list subcommands",
            "Add org-level listing via sgh org list command",
            "Add comprehensive error messages with resolution guidance",
            "Support concurrent processing with --workers flag defaulting to 5"
        )
    }
)

# STEP 5: Create Stories + Sub-tasks
Write-Host "[4/5] Creating User Stories and Sub-tasks" -ForegroundColor Cyan
$allKeys = [System.Collections.Generic.List[string]]::new()

foreach ($us in $stories) {
    $sf = @{
        project          = @{ key = $PROJECT_KEY }
        issuetype        = @{ id  = $storyType.id }
        summary          = $us.s
        customfield_10014 = $EPIC_KEY
        description = @{
            type = "doc"; version = 1
            content = @( @{ type = "paragraph"; content = @( @{ type = "text"; text = $us.d } ) } )
        }
    }
    $story = Invoke-Jira "POST" "/rest/api/3/issue" @{ fields = $sf }
    if (-not $story) { Write-Host "  Skipped story: $($us.s)" -ForegroundColor Yellow; continue }
    $sk = $story.key
    Write-Host "  Story $sk : $($us.s)" -ForegroundColor White
    $allKeys.Add($sk)

    foreach ($taskText in $us.t) {
        $tf = @{
            project   = @{ key = $PROJECT_KEY }
            issuetype = @{ id  = $subtaskType.id }
            summary   = $taskText
            parent    = @{ key = $sk }
        }
        $task = Invoke-Jira "POST" "/rest/api/3/issue" @{ fields = $tf }
        if ($task) {
            $allKeys.Add($task.key)
            Write-Host "    Sub-task $($task.key): $taskText" -ForegroundColor DarkGray
        } else {
            Write-Host "    Failed sub-task: $taskText" -ForegroundColor Yellow
        }
    }
}

$allKeys.Add($EPIC_KEY)

# STEP 6: Mark all Done
Write-Host "[5/5] Marking all issues as Done" -ForegroundColor Cyan
$doneCount = 0; $skipCount = 0

foreach ($k in $allKeys) {
    $tid = Get-DoneTransitionId $k
    if ($tid) {
        Invoke-Jira "POST" "/rest/api/3/issue/$k/transitions" @{ transition = @{ id = $tid } } | Out-Null
        Write-Host "  Done: $k" -ForegroundColor Green
        $doneCount++
    } else {
        Write-Host "  Skip: $k (no transition)" -ForegroundColor Yellow
        $skipCount++
    }
}

$storyCount   = $stories.Count
$subtaskCount = $allKeys.Count - $storyCount - 1

Write-Host ""
Write-Host "All done! Summary:" -ForegroundColor Cyan
Write-Host "  Project   : $PROJECT_KEY (company-managed)" -ForegroundColor Green
Write-Host "  Epic      : $EPIC_KEY" -ForegroundColor Green
Write-Host "  Stories   : $storyCount" -ForegroundColor Green
Write-Host "  Sub-tasks : $subtaskCount" -ForegroundColor Green
Write-Host "  Marked Done : $doneCount  |  Skipped: $skipCount" -ForegroundColor Green
Write-Host "  URL: $JIRA_URL/jira/software/projects/$PROJECT_KEY/boards" -ForegroundColor Cyan
