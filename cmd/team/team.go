package team

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/team"
	"github.com/prady-lab/sgh-cli/pkg/ui"
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

	teamCmd.AddCommand(listCommand(ctx))
	return teamCmd
}

var (
	allMembers  bool
	noOfMembers int
	teamName    string
)

func listCommand(ctx *context.Context) *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List teams and corresponding members in each team",
		Long: `List teams and corresponding members in each team for the given owner/organization.
By default, it will list 50 members in each team.`,

		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh team list --org <owner>
			$ sgh team list --org <owner> --team <team> --all-members
		`),

		Args: func(cmd *cobra.Command, args []string) error {
			if allMembers && teamName == "" {
				return fmt.Errorf("team name is required when listing all members")
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
			if ctx.JSON {
				ui.PrintJSON(teams)
				return
			}
			ui.PrintTeams(teams)
		},
	}

	listCmd.Flags().StringVarP(&teamName, "team", "t", "", "team name")
	listCmd.Flags().IntVarP(&noOfMembers, "members", "n", 50, "number of members to list in each team")
	listCmd.Flags().BoolVarP(&allMembers, "all-members", "a", false, "list all members in the team")
	listCmd.MarkPersistentFlagRequired("org")

	return listCmd
}
