/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
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
	logger "github.com/prady-lab/sgh-cli/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewRootCommand(ctx *context.Context) *cobra.Command {

	var rootCmd = &cobra.Command{
		Use:   "sgh <command> <subcommand> [flags]",
		Short: "Simple GitHub CLI",
		Long:  `Simple CLI to process the all or selected repositories in an organization.`,
		Example: heredoc.Doc(`
				$ sgh branch create
				$ sgh tag create
				$ sgh pb update --org sample-org -r sample-repo1 --branch sample-branch  -l -d
				$ sgh pr list --org sample-org -r sample-repo1 -r sample-repo2 --base "develop"
				$ sgh post-release -o sample-org -r sample-repo1 -r sample-repo2 --base "main" --head "Release-1.0" --create-tag
			`),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			if orgName == "" && (!cmd.HasParent() || (cmd.HasParent() && cmd.Parent().Name() != "config" && cmd.Parent().Name() != "repo")) {
				fmt.Println(`Error: required flag(s) "organization name" not set`)
				cmd.Help()
				os.Exit(1)
			}
			verbose, _ := cmd.Flags().GetBool("verbose")
			ctx.SetVerbose(verbose)
			logResponse, _ := cmd.Flags().GetBool("log-response")
			ctx.SetLogResponse(logResponse)
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
