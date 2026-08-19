// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/pradyb/sgh-cli/cmd/branch"
	"github.com/pradyb/sgh-cli/cmd/commit"
	"github.com/pradyb/sgh-cli/cmd/issue"
	cmdorg "github.com/pradyb/sgh-cli/cmd/org"
	"github.com/pradyb/sgh-cli/cmd/pr"
	protectedbranch "github.com/pradyb/sgh-cli/cmd/protectedbranch"
	"github.com/pradyb/sgh-cli/cmd/repo"
	"github.com/pradyb/sgh-cli/cmd/security"
	"github.com/pradyb/sgh-cli/cmd/tag"
	"github.com/pradyb/sgh-cli/cmd/team"
	"github.com/pradyb/sgh-cli/cmd/whoami"
	"github.com/pradyb/sgh-cli/cmd/workflow"
	"github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/ui"
	"github.com/spf13/cobra"
)

type shortcut struct {
	name    string
	expands string
	group   string
	builder func(*context.Context) *cobra.Command
}

// shortcutGroups controls the display order in `sgh shortcuts`.
var shortcutGroups = []string{
	"Repository", "Organization", "Issue", "Pull Request", "Branch", "Tag",
	"Protected Branch", "Workflow", "Commit", "Team", "Security", "Config", "Utilities",
}

var shortcutDefs = func(ctx *context.Context) []shortcut {
	return []shortcut{
		{"rpl", "repo list", "Repository", repo.ListCommand},
		{"rps", "repo search", "Repository", repo.SearchCommand},
		{"rpa", "repo archive", "Repository", repo.ArchiveCommand},
		{"rpv", "repo visibility", "Repository", repo.VisibilityCommand},
		{"orl", "org list", "Organization", cmdorg.ListCommand},
		{"isl", "issue list", "Issue", issue.ListCommand},
		{"isv", "issue view", "Issue", issue.ViewCommand},
		{"isc", "issue create", "Issue", issue.CreateCommand},
		{"prl", "pr list", "Pull Request", pr.ListCommand},
		{"prv", "pr view", "Pull Request", pr.ViewCommand},
		{"prc", "pr create", "Pull Request", pr.CreateCommand},
		{"prx", "pr close", "Pull Request", pr.CloseCommand},
		{"brl", "branch list", "Branch", branch.ListCommand},
		{"brc", "branch create", "Branch", branch.CreateCommand},
		{"brr", "branch rename", "Branch", branch.RenameCommand},
		{"brd", "branch delete", "Branch", branch.DeleteCommand},
		{"tgl", "tag list", "Tag", tag.ListCommand},
		{"tgc", "tag create", "Tag", tag.CreateCommand},
		{"tgd", "tag delete", "Tag", tag.DeleteCommand},
		{"pbl", "protected-branch list", "Protected Branch", protectedbranch.ListCommand},
		{"wfl", "workflow list", "Workflow", workflow.ListCommand},
		{"wfv", "workflow view", "Workflow", workflow.ViewCommand},
		{"wfd", "workflow dispatch", "Workflow", workflow.DispatchCommand},
		{"cil", "commit list", "Commit", commit.ListCommand},
		{"tml", "team list", "Team", team.ListCommand},
		{"secl", "security list", "Security", security.ListAlertsCommand},
		{"wai", "whoami", "Utilities", func(ctx *context.Context) *cobra.Command { return whoami.NewWhoAmICommand(ctx) }},
	}
}

func registerShortcuts(rootCmd *cobra.Command, ctx *context.Context) {
	for _, s := range shortcutDefs(ctx) {
		cmd := s.builder(ctx)
		cmd.Use = s.name
		cmd.Short = fmt.Sprintf("→ %s", s.expands)
		cmd.GroupID = "shortcuts"
		rootCmd.AddCommand(cmd)
	}
}

func newShortcutsHelpCommand(ctx *context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "shortcuts",
		Short: "List available command shortcuts",
		Long:  "Display all single-word shortcuts that expand to two-word commands.",
		Run: func(cmd *cobra.Command, args []string) {
			headerStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.Cyan)
			shortcutStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.Green).Width(8)
			arrowStyle := lipgloss.NewStyle().Foreground(ui.Dimmed)
			expandStyle := lipgloss.NewStyle().Foreground(ui.White)
			groupStyle := lipgloss.NewStyle().Foreground(ui.Subtle).Italic(true)

			fmt.Println()
			fmt.Println(headerStyle.Render("  Available Shortcuts"))
			fmt.Println(headerStyle.Render("  ───────────────────"))
			fmt.Println()

			grouped := make(map[string][]shortcut)
			for _, s := range shortcutDefs(ctx) {
				grouped[s.group] = append(grouped[s.group], s)
			}

			for _, g := range shortcutGroups {
				items := grouped[g]
				if len(items) == 0 {
					continue
				}
				fmt.Printf("  %s\n", groupStyle.Render(g))
				for _, s := range items {
					fmt.Printf("    %s %s %s\n",
						shortcutStyle.Render(s.name),
						arrowStyle.Render("→"),
						expandStyle.Render(s.expands),
					)
				}
			}
			fmt.Println()
			fmt.Println(arrowStyle.Render("  Usage: sgh <shortcut> [flags]"))
			fmt.Println(arrowStyle.Render("  Example: sgh prl --org my-org"))
			fmt.Println(arrowStyle.Render("  Help: sgh <shortcut> --help"))
			fmt.Println()
		},
	}
}
