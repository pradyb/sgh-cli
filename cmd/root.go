package cmd

import (
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/prady-lab/sgh-cli/cmd/branch"
	"github.com/prady-lab/sgh-cli/cmd/clone"
	"github.com/prady-lab/sgh-cli/cmd/commit"
	"github.com/prady-lab/sgh-cli/cmd/config"
	postrelease "github.com/prady-lab/sgh-cli/cmd/post_release"
	"github.com/prady-lab/sgh-cli/cmd/pr"
	protectedbranch "github.com/prady-lab/sgh-cli/cmd/protected_branch"
	"github.com/prady-lab/sgh-cli/cmd/repo"
	"github.com/prady-lab/sgh-cli/cmd/tag"
	"github.com/prady-lab/sgh-cli/cmd/team"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
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
			if cmd.HasParent() && (cmd.Parent().Name() == "config" || cmd.Parent().Name() == "repo") {
				cmd.InheritedFlags().SetAnnotation("org", cobra.BashCompOneRequiredFlag, []string{"false"})
			}
			verbose, _ := cmd.Flags().GetBool("verbose")
			ctx.SetVerbose(verbose)
			logResponse, _ := cmd.Flags().GetBool("log-response")
			ctx.SetLogResponse(logResponse)
			workers, _ := cmd.Flags().GetInt("workers")
			ctx.SetWorkerCount(workers)
			userFlags := make([]string, 0)
			flags := cmd.Flags()
			flags.VisitAll(func(f *pflag.Flag) {
				if f.Changed {
					userFlags = append(userFlags, "--"+f.Name+" "+f.Value.String())
				}
			})
			logger.Flog.Info().Msgf("Processing command: %s %s", cmd.CommandPath(), strings.Join(userFlags, " "))
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().StringP("org", "o", "", "organization name")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolP("log-response", "L", false, "log HTTP response")
	rootCmd.PersistentFlags().IntP("workers", "w", 5, "number of workers")
	rootCmd.MarkPersistentFlagRequired("org")

	rootCmd.AddCommand(config.NewConfigCommand(ctx))
	rootCmd.AddCommand(repo.NewRepoCommand(ctx))
	rootCmd.AddCommand(branch.NewBranchCommand(ctx))
	rootCmd.AddCommand(tag.NewTagCommand(ctx))
	rootCmd.AddCommand(pr.NewPRCommand(ctx))
	rootCmd.AddCommand(protectedbranch.NewProtectedBranchCommand(ctx))
	rootCmd.AddCommand(postrelease.NewPostReleaseCommand(ctx))
	rootCmd.AddCommand(commit.NewCommitCommand(ctx))
	rootCmd.AddCommand(clone.NewCloneCommand(ctx))
	rootCmd.AddCommand(team.NewTeamCommand(ctx))

	return rootCmd
}
