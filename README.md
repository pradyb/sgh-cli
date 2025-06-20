# 🚀 Simple GitHub CLI (sgh)

A powerful command-line tool for managing GitHub repositories at scale. Perform bulk operations on branches, tags, pull requests, protected branches, and more across your entire GitHub organization with a single command.

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

### From Source
```bash
git clone https://github.com/prady-lab/sgh-cli.git
cd sgh-cli
go build -o sgh .
```

### Prerequisites
- Go 1.19 or higher
- GitHub Personal Access Token with appropriate permissions:
  - `repo` - Full repository access
  - `admin:org` - Organization administration (for team operations)
  - `delete_repo` - Repository deletion (if needed)

## 🔐 Authentication

Create a GitHub Personal Access Token:
1. Go to GitHub Settings → Developer settings → Personal access tokens
2. Click "Generate new token"
3. Select the required scopes (see Prerequisites above)
4. Copy the token and set it as an environment variable:
   ```bash
   export GITHUB_TOKEN=your_token_here
   ```

## ⚡ Performance Tips

- **Adjust worker count** based on your rate limits and network capacity
- **Use specific repository filters** to avoid processing unnecessary repos
- **Enable verbose mode** (`-v`) for detailed operation logs
- **Use configuration files** to avoid repeating common parameters

## 🐛 Troubleshooting

### Common Issues

**Rate Limiting:**
```bash
# Reduce worker count to avoid rate limits
sgh pr create --org my-org --workers 2 --title "Update"
```

**Permission Errors:**
- Ensure your GitHub token has the required scopes
- Verify you have admin access to the organization/repositories

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

### Development Setup
```bash
git clone https://github.com/prady-lab/sgh-cli.git
cd sgh-cli
go mod download
go build -o sgh .
```

### Running Tests
```bash
go test ./...
```

## 📚 Available Commands

### Repository Management
- `repo` - List and manage repositories
- `clone` - Clone multiple repositories at once
- `commit` - Track and list commits across repositories

### Branch & Tag Operations
- `branch` - Create and delete branches across repositories
- `tag` - Create and delete tags across repositories
- `pb` - Manage protected branch settings

### Pull Request Workflows
- `pr` - Create, list, update, and merge pull requests
- `post-release` - Automate post-release workflows

### Organization Management
- `team` - List teams and members
- `config` - Manage CLI configuration

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

# List PRs for specific repositories
sgh pr list --org my-org --repo app1 --repo app2 --base main --state open

# Merge PRs in bulk
sgh pr merge --org my-org --pr-number 123 --repo my-app --merge-method squash
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

# Track recent commits
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

## 🆘 Getting Help

For detailed help on any command:
```bash
sgh <command> --help
sgh <command> <subcommand> --help
```

Examples:
```bash
sgh branch --help
sgh pr create --help
sgh config add --help
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