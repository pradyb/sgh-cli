// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package audit

import (
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	pkgaudit "github.com/pradyb/sgh-cli/pkg/audit"
	"github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/ui"
)

func NewAuditCommand(ctx *context.Context) *cobra.Command {
	auditCmd := &cobra.Command{
		Use:     "audit <command>",
		Aliases: []string{"al"},
		Short:   "View organization audit log",
		Long:    `Access and filter the GitHub organization audit log.`,
	}
	auditCmd.AddCommand(listCommand(ctx))
	return auditCmd
}

func listCommand(ctx *context.Context) *cobra.Command {
	var (
		phrase  string
		include string
		count   int
	)

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List audit log entries",
		Long: `List audit log entries for the organization.
Requires the organization to have an audit log enabled (GitHub Enterprise or GitHub.com orgs).`,
		Aliases: []string{"ls"},
		Example: heredoc.Doc(`
			$ sgh audit list --org my-org
			$ sgh audit list --org my-org --count 50
			$ sgh audit list --org my-org --phrase "repo.create"
			$ sgh audit list --org my-org --include "git"
		`),
		Run: func(cmd *cobra.Command, args []string) {
			orgName, _ := cmd.Flags().GetString("org")
			compact, _ := cmd.Flags().GetBool("compact")
			resp := pkgaudit.ListAuditLog(ctx, pkgaudit.AuditListRequest{
				OrgName: orgName,
				Phrase:  phrase,
				Include: include,
				Count:   count,
			})
			if resp.ErrorMessage != "" {
				fmt.Fprintln(os.Stderr, "Error: "+resp.ErrorMessage)
				os.Exit(1)
			}
			if ctx.JSON {
				ui.PrintJSON(resp.Entries)
				return
			}
			ui.PrintAuditLog(resp.Entries, compact)
		},
	}

	listCmd.Flags().StringVarP(&phrase, "phrase", "p", "", "filter by action phrase (e.g. repo.create)")
	listCmd.Flags().StringVarP(&include, "include", "i", "", "event type filter: web, git, or all (default: all)")
	listCmd.Flags().IntVarP(&count, "count", "c", 30, "number of entries to fetch")
	return listCmd
}
