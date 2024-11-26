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
		Short: "Simple GitHub Command Line Interface",
		Long:  `Simple CLI to process the all or selected repositories in an organization.`,
		Example: heredoc.Doc(`
				$ sgh branch create --org sample-org --new Release-1.1 --ref Release-1.0 
				$ sgh tag create --org sample-org --tag Release-1.0 --head Release-1.0 --message 'Tag for Release 1.0'
				$ sgh pb update --org sample-org --branch sample-branch --repo sample-repo1 -l -d --add-bypass-user john-doe --add-push-user jane-doe
				$ sgh pr list --org sample-org --repo sample-repo1 --repo sample-repo2 --base "develop"
				$ sgh post-release --org sample-org --base "main" --head "Release-1.0" --create-tag --title "Release 1.0"
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
