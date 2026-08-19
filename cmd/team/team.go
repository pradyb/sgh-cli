// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package team

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/logger"
	"github.com/pradyb/sgh-cli/pkg/team"
	"github.com/pradyb/sgh-cli/pkg/ui"
)

func NewTeamCommand(ctx *context.Context) *cobra.Command {
	teamCmd := &cobra.Command{
		Use:   "team <command>",
		Short: "List teams and corresponding members in each team",
		Long:  `List teams and corresponding members in each team for the given owner/organization.`,
		Example: heredoc.Doc(`
			$ sgh team list --org <owner>
		`),
	}

	teamCmd.AddCommand(ListCommand(ctx))
	return teamCmd
}

var (
	allMembers  bool
	noOfMembers int
	teamName    string
)

func ListCommand(ctx *context.Context) *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List teams and corresponding members in each team",
		Long: `List teams and corresponding members in each team for the given owner/organization.
By default, it will list 50 members in each team.`,

		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh team list --org <owner>
			$ sgh team list --org <owner> --team <team> --all
		`),

		Args: func(cmd *cobra.Command, args []string) error {
			// support deprecated --all-members alias
			if v, _ := cmd.Flags().GetBool("all-members"); v {
				allMembers = true
			}
			if allMembers && teamName == "" {
				return fmt.Errorf("team name is required when --all is used")
			}
			if noOfMembers > 100 {
				return fmt.Errorf("maximum number of members to list is 100")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			req := team.TeamMembersRequest{
				OrgName:     orgName,
				TeamName:    teamName,
				NoOfMembers: noOfMembers,
				AllMembers:  allMembers,
			}
			teams, err := team.GetTeamAndMembers(ctx, req)
			if err != nil {
				logger.Glog.Err(err).Msg("Error getting team and members")
				return
			}
			if ctx.Limit > 0 && len(teams) > ctx.Limit {
				teams = teams[:ctx.Limit]
			}
			if ctx.JSON {
				ui.PrintJSON(teams)
				return
			}
			ui.PrintTeams(teams)
		},
	}

	listCmd.Flags().StringVarP(&teamName, "team", "t", "", "team name")
	listCmd.Flags().IntVarP(&noOfMembers, "members", "n", 50, "number of members to list in each team")
	listCmd.Flags().BoolVarP(&allMembers, "all", "a", false, "list all members in the team (requires --team)")
	listCmd.Flags().Bool("all-members", false, "alias for --all (deprecated)")
	listCmd.Flags().MarkHidden("all-members")

	return listCmd
}
