package team

import (
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/team"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	logger "github.com/prady-lab/sgh-cli/utils"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

func NewTeamCommand(ctx *context.Context) *cobra.Command {

	var teamCmd = &cobra.Command{
		Use:   "team <command>",
		Short: "Organization teams",
		Long:  `Organization teams`,
		Example: heredoc.Doc(`
			$ sgh team list --org <owner>
		`),
	}

	teamCmd.AddCommand(listCommand(ctx))
	return teamCmd
}

var allMembers bool
var noOfMembers int
var teamName string

func listCommand(ctx *context.Context) *cobra.Command {
	var listCmd = &cobra.Command{
		Use:   "list",
		Short: "List team and members for the given owner/organization",
		Long: `List team and members for the given owner/organization.
By default, it will list 50 members in each team.`,

		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh team list --org <owner>
			$ sgh team list --org <owner> --team <team> --all-members
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			teams, err := team.GetTeamAndMembers(ctx, orgName, teamName, noOfMembers, allMembers)
			if err != nil {
				logger.Glog.Err(err).Msg("Error getting team and members")
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
