package security

import (
	"fmt"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/security"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/prady-lab/sgh-cli/utils"
)

var (
	repoNames        []string
	excludeRepoNames []string
	alertState       string
	secretType       string
	sortBy           string
)

func NewSecurityCommand(ctx *context.Context) *cobra.Command {
	securityCmd := &cobra.Command{
		Use:     "security <command>",
		Aliases: []string{"sec"},
		Short:   "Manage security operations like secret scanning alerts",
		Long: heredoc.Doc(`
			Manage security operations including secret scanning alerts across repositories.

			Available Operations:
			  list     List secret scanning alerts across repositories
			  view     View detailed alert information
			  update   Update/resolve security alerts

			Quick Filters (list command):
			  --state open       Show only open alerts
			  --state resolved   Show only resolved alerts
			  --secret-type      Filter by specific secret type (e.g., aws_access_key, github_token)
		`),
		Example: heredoc.Doc(`
			List all secret scanning alerts:
			  $ sgh security list --org my-org

			List only open alerts:
			  $ sgh security list --org my-org --state open

			Filter by secret type:
			  $ sgh security list --org my-org --secret-type aws_access_key

			View alert details:
			  $ sgh security view --org my-org -r my-app --alert 1

			Resolve an alert:
			  $ sgh security update --org my-org -r my-app --alert 1 --state resolved --resolution false_positive
		`),
	}

	securityCmd.AddCommand(ListAlertsCommand(ctx))
	securityCmd.AddCommand(ViewAlertCommand(ctx))
	securityCmd.AddCommand(UpdateAlertCommand(ctx))
	return securityCmd
}

func ListAlertsCommand(ctx *context.Context) *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List secret scanning alerts across repositories",
		Long: `List secret scanning alerts for given repos or all selected repos in the organization.
Supports filtering by state (open, resolved) and secret type.`,
		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh security list --org sample-org
			$ sgh security list --org sample-org --state open
			$ sgh security list --org sample-org --state resolved
			$ sgh security list --org sample-org -r sample-repo1 -r sample-repo2
			$ sgh security list --org sample-org --secret-type aws_access_key
			$ sgh security list --org sample-org --sort state
		`),

		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")

			if alertState != "" && alertState != "open" && alertState != "resolved" {
				fmt.Println(ui.ErrorMessage("Invalid state: %s. Must be one of: open, resolved", alertState))
				return
			}

			req := security.AlertListRequest{
				OrgName:          orgName,
				RepoNames:        repoNames,
				ExcludeRepoNames: excludeRepoNames,
				State:            alertState,
				SecretType:       secretType,
			}
			alerts := security.ListSecretScanningAlerts(ctx, req)
			ui.SortSecretAlerts(alerts, sortBy)
			if ctx.Limit > 0 && len(alerts) > ctx.Limit {
				alerts = alerts[:ctx.Limit]
			}
			if ctx.JSON {
				ui.PrintJSON(alerts)
				return
			}
			ui.PrintSecretScanningAlerts(alerts, sortBy, ctx.Compact)
		},
	}

	listCmd.Flags().StringArrayVarP(&repoNames, "repository", "r", []string{}, "repository names to include")
	listCmd.Flags().StringArrayVarP(&excludeRepoNames, utils.EXCLUDE_REPOSITORY_FLAG, "e", []string{}, "repository names to exclude")
	listCmd.Flags().StringVarP(&alertState, "state", "s", "", "filter by `state`: open, resolved")
	listCmd.Flags().StringVar(&secretType, "secret-type", "", "filter by secret `type` (e.g., aws_access_key, github_token)")
	listCmd.Flags().StringVar(&sortBy, "sort", "", "sort results by: repo, state, type, created")

	listCmd.MarkPersistentFlagRequired("org")
	return listCmd
}

func ViewAlertCommand(ctx *context.Context) *cobra.Command {
	var viewRepo string
	var alertNumber int

	viewCmd := &cobra.Command{
		Use:     "view",
		Short:   "View secret scanning alert details",
		Long:    `View detailed information about a specific secret scanning alert.`,
		Aliases: []string{"detail", "info"},
		Example: heredoc.Doc(`
			$ sgh security view --org sample-org -r sample-repo --alert 1
			$ sgh security view --org sample-org -r sample-repo --alert 5 --json
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")

			req := security.AlertViewRequest{
				OrgName:     orgName,
				RepoName:    viewRepo,
				AlertNumber: alertNumber,
			}
			alert := security.GetSecretScanningAlert(ctx, req)
			if ctx.JSON {
				ui.PrintJSON(alert)
				return
			}
			ui.PrintSecretAlertDetail(alert)
		},
	}

	viewCmd.Flags().StringVarP(&viewRepo, "repository", "r", "", "repository `name`")
	viewCmd.Flags().IntVarP(&alertNumber, "alert", "a", 0, "alert `number`")

	viewCmd.MarkPersistentFlagRequired("org")
	viewCmd.MarkFlagRequired("repository")
	viewCmd.MarkFlagRequired("alert")

	return viewCmd
}

func UpdateAlertCommand(ctx *context.Context) *cobra.Command {
	var updateRepo string
	var alertNumber int
	var updateState string
	var resolution string
	var resolutionComment string

	updateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update or resolve a secret scanning alert",
		Long: `Update the state of a secret scanning alert or mark it as resolved.
Valid resolutions: false_positive, wont_fix, revoked, used_in_tests`,
		Aliases: []string{"resolve"},
		Example: heredoc.Doc(`
			$ sgh security update --org sample-org -r sample-repo --alert 1 --state resolved --resolution false_positive
			$ sgh security update --org sample-org -r sample-repo --alert 1 --state resolved --resolution revoked --comment "Key has been rotated"
			$ sgh security update --org sample-org -r sample-repo --alert 1 --state open
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")

			if updateState != "open" && updateState != "resolved" {
				fmt.Println(ui.ErrorMessage("Invalid state: %s. Must be one of: open, resolved", updateState))
				return
			}

			if updateState == "resolved" {
				if resolution == "" {
					fmt.Println(ui.ErrorMessage("--resolution is required when state is 'resolved'"))
					return
				}
				validResolutions := map[string]bool{
					"false_positive": true,
					"wont_fix":       true,
					"revoked":        true,
					"used_in_tests":  true,
				}
				if !validResolutions[resolution] {
					fmt.Println(ui.ErrorMessage("Invalid resolution: %s. Must be one of: false_positive, wont_fix, revoked, used_in_tests", resolution))
					return
				}
			}

			if ctx.DryRun {
				ui.PrintDryRunBanner()
				details := map[string]string{
					"Alert Number": fmt.Sprintf("%d", alertNumber),
					"New State":    updateState,
				}
				if resolution != "" {
					details["Resolution"] = resolution
				}
				if resolutionComment != "" {
					details["Comment"] = resolutionComment
				}
				repos, _ := processor.ResolveRepositoryNames(ctx, orgName, []string{updateRepo}, nil)
				ui.PrintDryRunActions("Update Security Alert", orgName, repos, details)
				return
			}

			req := security.AlertUpdateRequest{
				OrgName:           orgName,
				RepoName:          updateRepo,
				AlertNumber:       alertNumber,
				State:             updateState,
				Resolution:        resolution,
				ResolutionComment: resolutionComment,
			}
			alert := security.UpdateSecretScanningAlert(ctx, req)
			if alert.ErrorMessage != "" {
				fmt.Println(ui.ErrorMessage("%s", alert.ErrorMessage))
				return
			}
			if ctx.JSON {
				ui.PrintJSON(alert)
				return
			}
			fmt.Printf("  Successfully updated alert #%d in %s\n", alertNumber, alert.RepositoryName)
			ui.PrintSecretAlertDetail(alert)
		},
	}

	updateCmd.Flags().StringVarP(&updateRepo, "repository", "r", "", "repository `name`")
	updateCmd.Flags().IntVarP(&alertNumber, "alert", "a", 0, "alert `number`")
	updateCmd.Flags().StringVarP(&updateState, "state", "s", "", "`state` to set: open, resolved")
	updateCmd.Flags().StringVar(&resolution, "resolution", "", "`resolution` reason: false_positive, wont_fix, revoked, used_in_tests")
	updateCmd.Flags().StringVarP(&resolutionComment, "comment", "c", "", "optional resolution `comment`")

	updateCmd.MarkPersistentFlagRequired("org")
	updateCmd.MarkFlagRequired("repository")
	updateCmd.MarkFlagRequired("alert")
	updateCmd.MarkFlagRequired("state")

	return updateCmd
}
