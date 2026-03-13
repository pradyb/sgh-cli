// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package whoami

import (
	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/prady-lab/sgh-cli/pkg/whoami"
)

func NewWhoAmICommand(ctx *context.Context) *cobra.Command {
	return &cobra.Command{
		Use:     "whoami",
		Aliases: []string{"me"},
		Short:   "Show the authenticated GitHub user",
		Long: heredoc.Doc(`
			Display profile information for the currently authenticated GitHub user.

			Calls GET /user using the configured GITHUB_TOKEN and shows:
			  login, name, email, company, location, bio, public repos,
			  followers, following, profile URL, and member-since date.

			If your token has the 'user' scope, private repo count, disk
			usage, and plan name are also shown.

			No --org flag required.
		`),
		Example: heredoc.Doc(`
			$ sgh whoami
			$ sgh me
			$ sgh whoami --json
		`),
		Run: func(cmd *cobra.Command, args []string) {
			user := whoami.GetCurrentUser(ctx)
			if ctx.JSON {
				ui.PrintJSON(user)
				return
			}
			ui.PrintWhoAmI(user)
		},
	}
}
