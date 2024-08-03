/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/prady-lab/sgh-cli/cmd/branch"
	"github.com/prady-lab/sgh-cli/cmd/config"
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
				$ sgh config set
			`),
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}

	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentFlags().StringP("org", "o", "", "organization name")

	rootCmd.AddCommand(config.NewConfigCommand(ctx))
	rootCmd.AddCommand(repo.NewRepoCommand(ctx))
	rootCmd.AddCommand(branch.NewBranchCommand(ctx))
	rootCmd.AddCommand(tag.NewTagCommand(ctx))

	return rootCmd
}
