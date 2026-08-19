// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package org

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/logger"
	"github.com/pradyb/sgh-cli/pkg/org"
	"github.com/pradyb/sgh-cli/pkg/ui"
)

func NewOrgCommand(ctx *context.Context) *cobra.Command {
	orgCmd := &cobra.Command{
		Use:   "org <command>",
		Short: "View GitHub organization details",
		Long:  `View detailed information about all GitHub organizations the token belongs to.`,
		Example: heredoc.Doc(`
			$ sgh org list
			$ sgh orl
		`),
	}
	orgCmd.AddCommand(ListCommand(ctx))
	return orgCmd
}

func ListCommand(ctx *context.Context) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List all GitHub organizations the token belongs to",
		Aliases: []string{"ls"},
		Long: heredoc.Doc(`
			Fetch and display rich details for all GitHub organizations the
			authenticated token belongs to, using the GraphQL API.

			No --org flag needed — results come from viewer.organizations.

			Fields returned:
			  Name, Login, Description, Email, Location, Website, Members, Teams,
			  Repositories (public + private), Verified, 2FA Required, Disk Usage, Created.
		`),
		Example: heredoc.Doc(`
			$ sgh org list
			$ sgh orl
			$ sgh orl -J     # JSON output
		`),
		Run: func(cmd *cobra.Command, args []string) {
			logger.Flog.Info().Msg("Listing organizations for authenticated token")

			details := org.ListOrgs(ctx)
			if ctx.JSON {
				ui.PrintJSON(details)
				return
			}
			ui.PrintOrganizations(details)
		},
	}
}
