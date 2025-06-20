# 🚀 Simple GitHub CLI (sgh)

A powerful command-line tool for managing GitHub repositories at scale. Perform bulk operations on branches, tags, pull requests, protected branches, and more across your entire GitHub organization with a single command.

## 📋 Table of Contents

- [✨ Key Features](#-key-features)
- [🚀 Quick Start](#-quick-start)
- [📦 Installation](#-installation)
- [🧪 Testing & Development](#-testing--development)
- [🔐 Authentication](#-authentication)
- [📚 Available Commands](#-available-commands)
- [🌟 Usage Examples](#-usage-examples)
- [🏷️ Global Flags](#️-global-flags)
- [🔍 Advanced Usage](#-advanced-usage)
- [⚡ Performance Tips](#-performance-tips)
- [🐛 Troubleshooting](#-troubleshooting)
- [📄 License](#-license)

## ✨ Key Features

- **Bulk Repository Operations**: Manage multiple repositories across entire organizations
- **Advanced Branch & Tag Management**: Create, delete, and manage branches and tags across repos
- **Pull Request Automation**: Create, list, update, and merge pull requests in bulk
- **Protected Branch Management**: Configure and update branch protection rules
- **Post-Release Workflows**: Automate post-release activities like merging and tagging
- **Team Management**: List teams and members across your organization
- **Repository Operations**: Clone repositories and track commits
- **Flexible Filtering**: Use include/exclude patterns to target specific repositories
- **Concurrent Processing**: Configurable worker threads for fast bulk operations

## 🚀 Quick Start

1. **Set your GitHub token:**
   ```bash
   export GITHUB_TOKEN=your_token_here
   ```

2. **List repositories in your organization:**
   ```bash
   sgh repo list your-org
   ```

3. **Create branches across repositories:**
   ```bash
   sgh branch create --org your-org --new feature-branch --ref main
   ```

4. **Bulk PR creation:**
   ```bash
   sgh pr create --org your-org --title "Feature update" --head feature-branch --base main
   ```

## 📦 Installation

### Option 1: From Source (Recommended)
```bash
git clone https://github.com/prady-lab/sgh-cli.git
cd sgh-cli
go build -o sgh .

# Add to PATH (optional)
# Linux/Mac:
sudo mv sgh /usr/local/bin/
# Windows: Move sgh.exe to a directory in your PATH
```

### Option 2: Go Install (if published)
```bash
go install github.com/prady-lab/sgh-cli@latest
```

### Option 3: Download Binary (if releases available)
Visit the [releases page](https://github.com/prady-lab/sgh-cli/releases) and download the appropriate binary for your platform.

### Prerequisites
- **Go 1.19 or higher** (for building from source)
- **GitHub Personal Access Token** with appropriate permissions:
  - `repo` - Full repository access
  - `admin:org` - Organization administration (for team operations)
  - `delete_repo` - Repository deletion (if needed)

### Verify Installation
```bash
sgh --help
```

## 🔐 Authentication

### Create a GitHub Personal Access Token:
1. Go to GitHub Settings → Developer settings → Personal access tokens
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

## ⚙️ Configuration

### Config File Locations
- **Windows:** `~/sgh.json`
- **Linux:** `~/.config/sgh/sgh.json`
- **Mac:** `~/.config/sgh/sgh.json`

### Configuration Management
```bash
# View current configuration
sgh config list

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
sgh repo list my-org --log-response --verbose
```

**Configuration Issues:**
```bash
# Check current configuration
sgh config list

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

## 📚 Available Commands

| Command | Subcommands | Description |
|---------|-------------|-------------|
| `repo` | `list` | List and manage repositories |
| `clone` | - | Clone multiple repositories at once |
| `commit` | `list` | Track and list commits across repositories |
| `branch` | `create`, `delete` | Create and delete branches across repositories |
| `tag` | `create`, `delete` | Create and delete tags across repositories |
| `pb` | `list`, `update`, `delete` | Manage protected branch settings |
| `pr` | `create`, `list`, `update`, `merge` | Create, list, update, and merge pull requests |
| `post-release` | - | Automate post-release workflows |
| `team` | `list` | List teams and members |
| `config` | `add`, `list` | Manage CLI configuration |

### Repository Management
- `repo list <owner>` - List repositories for an organization
- `clone --org <org> [--branch <branch>]` - Clone multiple repositories
- `commit list --org <org> [--days <n>]` - Track commits across repositories

### Branch & Tag Operations  
- `branch create --org <org> --new <name> --ref <ref>` - Create branches across repos
- `branch delete --org <org> --branch <name>` - Delete branches across repos
- `tag create --org <org> --tag <name> --head <ref> --message <msg>` - Create tags
- `tag delete --org <org> --tag <name>` - Delete tags

### Pull Request Workflows
- `pr create --org <org> --title <title> --head <branch> --base <branch>` - Create PRs
- `pr list --org <org> [--all-status]` - List pull requests
- `pr update --org <org> --repo <repo> --pr <number> --action <close|open>` - Update PRs
- `pr list --interactive` - Interactive PR management

### Protected Branch Management
- `pb list --org <org> --branch <branch>` - List protection settings
- `pb update --org <org> --branch <branch> [options]` - Update protection rules
- `pb delete --org <org> --branch <branch>` - Remove protection

### Organization Management
- `team list --org <org>` - List teams and members
- `config add <key> <value>` - Add configuration
- `config list` - Show current configuration

## 🌟 Usage Examples

### Branch Management
```bash
# Create a new release branch across all repos
sgh branch create --org my-org --new Release-1.1 --ref Release-1.0

# Create hotfix branch for specific repositories
sgh branch create --org my-org --new hotfix-branch --ref main --repo critical-app --repo important-service

# Delete old branches
sgh branch delete --org my-org --branch old-feature --repo legacy-app
```

### Tag Operations
```bash
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

# List PRs for specific repositories (using correct flag)
sgh pr list --org my-org --repo app1 --repo app2 --base main --all-status

# List PRs with filters
sgh pr list --org my-org --author john-doe --assignee jane-doe --last 10

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
sgh repo list my-org

# Clone repositories with specific branch
sgh clone --org my-org --branch develop

# Track recent commits (correct flags)
sgh commit list --org my-org --days 7 --details --include-merge-commits
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
# Add organization to config
sgh config add org my-org

# Add repository patterns
sgh config add pattern api-* --org my-org --include
sgh config add pattern legacy-* --org my-org --exclude

# List current configuration
sgh config list
```

## 🏷️ Global Flags

- `-h, --help` - Show help information
- `-o, --org string` - Organization name (required for most commands)
- `-v, --verbose` - Enable verbose output
- `-L, --log-response` - Log HTTP responses for debugging
- `-w, --workers int` - Number of concurrent workers (default: 5)

## 🔍 Advanced Usage

### Repository Filtering
Use include/exclude patterns to target specific repositories:

```bash
# Target only API services
sgh branch create --org my-org --new feature --ref main --repo api-*

# Exclude legacy applications
sgh pr create --org my-org --title "Update" --head feature --base main --exclude-repos legacy-*
```

### Concurrent Processing
Adjust worker count for optimal performance:

```bash
# Use more workers for faster processing
sgh clone --org large-org --workers 10

# Use fewer workers to avoid rate limiting
sgh pr create --org my-org --title "Update" --head feature --base main --workers 2
```

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
# Output: coverage: 81.2% of statements
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

**Coverage with specific package patterns:**
```bash
# Cover specific packages
go test -coverpkg=./internal/config,./pkg/config ./internal/config
```

### Development Guidelines

- Maintain **>80% test coverage** for core packages
- Add tests for new features and bug fixes
- Run tests before submitting pull requests
- Use the HTML coverage report to identify untested code paths

### Current Test Coverage Status

- ✅ `internal/config`: **81.2%** coverage (comprehensive test suite)
- ⚠️ Other packages: Tests needed

### Benchmark Tests

Run performance benchmarks:
```bash
go test ./internal/config -bench=.
go test ./internal/config -bench=BenchmarkIsOrganizationPresent
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) for CLI functionality
- Uses [GitHub REST API](https://docs.github.com/en/rest) and [GraphQL API](https://docs.github.com/en/graphql)
- Inspired by the need for bulk GitHub operations in large organizations

---

**Happy coding! 🚀**

For support, please open an issue on [GitHub](https://github.com/prady-lab/sgh-cli/issues).