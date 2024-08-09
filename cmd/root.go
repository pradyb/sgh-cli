/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
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
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/spf13/cobra"
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
			verbose, _ := cmd.Flags().GetBool("verbose")
			ctx.SetVerbose(verbose)
			logResponse, _ := cmd.Flags().GetBool("log-response")
			ctx.SetLogResponse(logResponse)
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().StringP("org", "o", "", "organization name")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().BoolP("log-response", "L", false, "Log HTTP response")

	rootCmd.AddCommand(config.NewConfigCommand(ctx))
	rootCmd.AddCommand(repo.NewRepoCommand(ctx))
	rootCmd.AddCommand(branch.NewBranchCommand(ctx))
	rootCmd.AddCommand(tag.NewTagCommand(ctx))
	rootCmd.AddCommand(pr.NewPRCommand(ctx))
	rootCmd.AddCommand(protectedbranch.NewProtectedBranchCommand(ctx))
	rootCmd.AddCommand(postrelease.NewPostReleaseCommand(ctx))
	rootCmd.AddCommand(commit.NewCommitCommand(ctx))
	rootCmd.AddCommand(clone.NewCloneCommand(ctx))

	return rootCmd
}
