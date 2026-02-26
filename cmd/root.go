package cmd

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/charmbracelet/lipgloss"
	"github.com/prady-lab/sgh-cli/cmd/branch"
	"github.com/prady-lab/sgh-cli/cmd/clone"
	"github.com/prady-lab/sgh-cli/cmd/commit"
	"github.com/prady-lab/sgh-cli/cmd/config"
	"github.com/prady-lab/sgh-cli/cmd/health"
	postrelease "github.com/prady-lab/sgh-cli/cmd/postrelease"
	"github.com/prady-lab/sgh-cli/cmd/pr"
	protectedbranch "github.com/prady-lab/sgh-cli/cmd/protectedbranch"
	"github.com/prady-lab/sgh-cli/cmd/repo"
	"github.com/prady-lab/sgh-cli/cmd/tag"
	"github.com/prady-lab/sgh-cli/cmd/team"
	"github.com/prady-lab/sgh-cli/cmd/version"
	"github.com/prady-lab/sgh-cli/cmd/workflow"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewRootCommand(ctx *context.Context) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "sgh <command> <subcommand> [flags]",
		Short: "🚀 Simple GitHub Command Line Interface",
		Long: heredoc.Doc(`
			🚀 Simple GitHub CLI (sgh) - A powerful tool for managing GitHub repositories at scale

			Manage multiple repositories across your GitHub organization with ease. Perform bulk operations 
			on branches, tags, pull requests, protected branches, and more with a single command.

			✨ Key Features:
			  • Bulk repository operations across entire organizations
			  • Advanced branch and tag management
			  • Pull request automation and management  
			  • Protected branch configuration and updates
			  • Post-release workflow automation
			  • Team and member management
			  • Repository cloning and commit tracking
			  • Flexible filtering with include/exclude patterns

			🔧 Configuration:
			  Environment Variables:
			    GITHUB_TOKEN    Your GitHub Personal Access Token (required)

			  Config Files:
			    Windows: ~/sgh.json
			    Linux:   ~/.config/sgh/sgh.json  
			    Mac:     ~/.config/sgh/sgh.json

			🎯 Quick Start:
			    1. Set your GitHub token: export GITHUB_TOKEN=your_token_here
			    2. List repositories: sgh repo list your-org
			    3. Create branches: sgh branch create --org your-org --new feature-branch --ref main
			    4. Bulk PR creation: sgh pr create --org your-org --title "Feature update" --head feature-branch --base main

			For detailed command help, use: sgh <command> --help
		`),

		Example: heredoc.Doc(`
			🌟 Common Workflows:

			Branch Management:
			  $ sgh branch create --org sample-org --new Release-1.1 --ref Release-1.0
			
			Tag Operations:
			  $ sgh tag create --org sample-org --tag Release-1.0 --head Release-1.0 --message 'Tag for Release 1.0'

			Protected Branch Management:
			  $ sgh pb update --org sample-org --branch sample-branch --repo sample-repo1 -l -d --add-bypass-user john-doe --add-push-user jane-doe

			Post-Release Workflows:
			  $ sgh post-release --org my-org --base main --head Release-1.0 --create-tag --title "Release 1.0"

			Repository Operations:
			  $ sgh repo list my-org
			  $ sgh clone --org my-org --branch develop
			  $ sgh commit list --org my-org --days 7 --details

			Team Management:
			  $ sgh team list --org my-org
			  $ sgh team list --org my-org --team developers --all-members

			Configuration:
			  $ sgh config add org my-org
			  $ sgh config add pattern api-* --org my-org --include
			`),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			validateCommandRequirements(cmd)
			setupContext(cmd, ctx)
			logCommandExecution(cmd)
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	rootCmd.CompletionOptions.DisableNoDescFlag = true
	rootCmd.PersistentFlags().StringP("org", "o", "", "organization name")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolP("log-response", "L", false, "log HTTP response")
	rootCmd.PersistentFlags().IntP("workers", "w", 5, "number of workers")
	rootCmd.PersistentFlags().BoolP("compact", "C", false, "minimal tab-separated output, suitable for piping")
	rootCmd.PersistentFlags().BoolP("json", "J", false, "output results as JSON for scripting")

	// Command groups for organized help output
	rootCmd.AddGroup(
		&cobra.Group{ID: "repo", Title: "Repository Management:"},
		&cobra.Group{ID: "git", Title: "Git Operations:"},
		&cobra.Group{ID: "cicd", Title: "CI/CD & Release:"},
		&cobra.Group{ID: "org", Title: "Organization:"},
		&cobra.Group{ID: "util", Title: "Utilities:"},
	)

	repoCmd := repo.NewRepoCommand(ctx)
	repoCmd.GroupID = "repo"
	cloneCmd := clone.NewCloneCommand(ctx)
	cloneCmd.GroupID = "repo"
	commitCmd := commit.NewCommitCommand(ctx)
	commitCmd.GroupID = "repo"

	branchCmd := branch.NewBranchCommand(ctx)
	branchCmd.GroupID = "git"
	tagCmd := tag.NewTagCommand(ctx)
	tagCmd.GroupID = "git"
	prCmd := pr.NewPRCommand(ctx)
	prCmd.GroupID = "git"
	pbCmd := protectedbranch.NewProtectedBranchCommand(ctx)
	pbCmd.GroupID = "git"

	workflowCmd := workflow.NewWorkflowCommand(ctx)
	workflowCmd.GroupID = "cicd"
	postreleaseCmd := postrelease.NewPostReleaseCommand(ctx)
	postreleaseCmd.GroupID = "cicd"

	teamCmd := team.NewTeamCommand(ctx)
	teamCmd.GroupID = "org"

	configCmd := config.NewConfigCommand(ctx)
	configCmd.GroupID = "util"
	healthCmd := health.NewHealthCommand(ctx)
	healthCmd.GroupID = "util"
	versionCmd := version.NewVersionCommand()
	versionCmd.GroupID = "util"

	rootCmd.AddCommand(repoCmd, cloneCmd, commitCmd)
	rootCmd.AddCommand(branchCmd, tagCmd, prCmd, pbCmd)
	rootCmd.AddCommand(workflowCmd, postreleaseCmd)
	rootCmd.AddCommand(teamCmd)
	rootCmd.AddCommand(configCmd, healthCmd, versionCmd)

	// Cobra auto-generates the completion command; assign it to the util group
	// so it appears alongside config, health, and version.
	rootCmd.InitDefaultCompletionCmd()
	if completionCmd, _, _ := rootCmd.Find([]string{"completion"}); completionCmd != nil && completionCmd.Name() == "completion" {
		completionCmd.GroupID = "util"
	}

	return rootCmd
}

// isValidOrgName validates GitHub organization name format
// GitHub org names can contain alphanumeric characters, hyphens, and underscores
// They cannot start or end with hyphens, and cannot be empty
func isValidOrgName(org string) bool {
	if org == "" {
		return false
	}
	// GitHub org name pattern: alphanumeric, hyphens, underscores, but cannot start/end with hyphen
	pattern := `^[a-zA-Z0-9]([a-zA-Z0-9_-]*[a-zA-Z0-9])?$`
	matched, err := regexp.MatchString(pattern, org)
	if err != nil {
		logger.Glog.Error().Err(err).Msg("Failed to validate organization name")
		return false
	}
	return matched && len(org) <= 39 // GitHub has a 39 character limit for org names
}

func validateOrganizationName(org string) error {
	if org == "" {
		return fmt.Errorf("organization name cannot be empty")
	}
	if !isValidOrgName(org) {
		return fmt.Errorf("organization name must contain only alphanumeric characters, hyphens, and underscores")
	}
	return nil
}

func validateWorkerCount(workers int) error {
	if workers < 1 {
		return fmt.Errorf("worker count must be at least 1")
	}
	if workers > 50 {
		return fmt.Errorf("worker count must be at most 50")
	}
	return nil
}

func validateCommandRequirements(cmd *cobra.Command) {
	cmdName := cmd.Name()
	parentName := ""
	if cmd.HasParent() {
		parentName = cmd.Parent().Name()
	}

	// Commands that don't require org parameter
	noOrgRequired := cmdName == "health" || cmdName == "version" ||
		parentName == "config" || parentName == "repo"

	// Validate org flag for commands that require it
	if !noOrgRequired {
		orgFlag, _ := cmd.Flags().GetString("org")
		if orgFlag == "" {
			logger.Glog.Error().Msg("Organization parameter is required for this command")
			printCLIError(
				fmt.Sprintf("Organization parameter is required for '%s' command.", cmdName),
				"Use --org or -o flag to specify the organization.",
			)
			os.Exit(1)
		}

		// Enhanced org name validation
		if err := validateOrganizationName(orgFlag); err != nil {
			logger.Glog.Error().
				Str("org", orgFlag).
				Err(err).
				Msg("Invalid organization name")
			printCLIError(
				fmt.Sprintf("Invalid organization name '%s': %v", orgFlag, err),
				"Organization names can only contain alphanumeric characters, hyphens, and underscores.",
			)
			os.Exit(1)
		}
	}

	// Enhanced worker count validation
	workers, _ := cmd.Flags().GetInt("workers")
	if err := validateWorkerCount(workers); err != nil {
		logger.Glog.Error().
			Int("workers", workers).
			Err(err).
			Msg("Invalid worker count")
		printCLIError(
			err.Error(),
			"Worker count must be between 1 and 50.",
		)
		os.Exit(1)
	}
}

func setupContext(cmd *cobra.Command, ctx *context.Context) {
	verbose, _ := cmd.Flags().GetBool("verbose")
	logger.SetVerbose(verbose)
	ctx.SetVerbose(verbose)

	logResponse, _ := cmd.Flags().GetBool("log-response")
	ctx.SetLogResponse(logResponse)

	workers, _ := cmd.Flags().GetInt("workers")
	ctx.SetWorkerCount(workers)

	compact, _ := cmd.Flags().GetBool("compact")
	ctx.Compact = compact

	jsonOutput, _ := cmd.Flags().GetBool("json")
	ctx.JSON = jsonOutput
}

func printCLIError(msg string, hint string) {
	errStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.Red)
	hintStyle := lipgloss.NewStyle().Foreground(ui.Dimmed).Italic(true)
	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, errStyle.Render("  ✗ "+msg))
	if hint != "" {
		fmt.Fprintln(os.Stderr, hintStyle.Render("    "+hint))
	}
	fmt.Fprintln(os.Stderr)
}

func logCommandExecution(cmd *cobra.Command) {
	userFlags := make([]string, 0)
	flags := cmd.Flags()
	flags.VisitAll(func(f *pflag.Flag) {
		if f.Changed {
			userFlags = append(userFlags, "--"+f.Name+" "+f.Value.String())
		}
	})
	logger.Flog.Info().Msgf("Processing command: %s %s", cmd.CommandPath(), strings.Join(userFlags, " "))
}
