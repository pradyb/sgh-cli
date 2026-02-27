package cmd

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/prady-lab/sgh-cli/cmd/branch"
	"github.com/prady-lab/sgh-cli/cmd/commit"
	"github.com/prady-lab/sgh-cli/cmd/pr"
	protectedbranch "github.com/prady-lab/sgh-cli/cmd/protectedbranch"
	"github.com/prady-lab/sgh-cli/cmd/repo"
	"github.com/prady-lab/sgh-cli/cmd/tag"
	"github.com/prady-lab/sgh-cli/cmd/team"
	"github.com/prady-lab/sgh-cli/cmd/workflow"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/spf13/cobra"
)

type shortcut struct {
	name    string
	expands string
	builder func(*context.Context) *cobra.Command
}

var shortcutDefs = func(ctx *context.Context) []shortcut {
	return []shortcut{
		{"prl", "pr list", pr.ListCommand},
		{"prv", "pr view", pr.ViewCommand},
		{"wfl", "workflow list", workflow.ListCommand},
		{"wfv", "workflow view", workflow.ViewCommand},
		{"brl", "branch list", branch.ListCommand},
		{"tgl", "tag list", tag.ListCommand},
		{"pbl", "pb list", protectedbranch.ListCommand},
		{"rpl", "repo list", repo.ListCommand},
		{"cil", "commit list", commit.ListCommand},
		{"tml", "team list", team.ListCommand},
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

func newShortcutsHelpCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "shortcuts",
		Short: "List available command shortcuts",
		Long:  "Display all single-word shortcuts that expand to two-word commands.",
		Run: func(cmd *cobra.Command, args []string) {
			headerStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.Cyan)
			shortcutStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.Green).Width(8)
			arrowStyle := lipgloss.NewStyle().Foreground(ui.Dimmed)
			expandStyle := lipgloss.NewStyle().Foreground(ui.White)

			fmt.Println()
			fmt.Println(headerStyle.Render("  Available Shortcuts"))
			fmt.Println(headerStyle.Render("  ───────────────────"))
			fmt.Println()

			groups := []struct {
				label string
				names []string
			}{
				{"Repository", []string{"rpl"}},
				{"Pull Request", []string{"prl", "prv"}},
				{"Branch", []string{"brl"}},
				{"Tag", []string{"tgl"}},
				{"Protected Branch", []string{"pbl"}},
				{"Workflow", []string{"wfl", "wfv"}},
				{"Commit", []string{"cil"}},
				{"Team", []string{"tml"}},
			}

			expansions := map[string]string{
				"prl": "pr list", "prv": "pr view",
				"wfl": "workflow list", "wfv": "workflow view",
				"brl": "branch list", "tgl": "tag list",
				"pbl": "pb list", "rpl": "repo list",
				"cil": "commit list", "tml": "team list",
			}

			groupStyle := lipgloss.NewStyle().Foreground(ui.Subtle).Italic(true)
			for _, g := range groups {
				fmt.Printf("  %s\n", groupStyle.Render(g.label))
				for _, name := range g.names {
					fmt.Printf("    %s %s %s\n",
						shortcutStyle.Render(name),
						arrowStyle.Render("→"),
						expandStyle.Render(expansions[name]),
					)
				}
			}
			fmt.Println()
			fmt.Println(arrowStyle.Render("  Usage: sgh <shortcut> [flags]"))
			fmt.Println(arrowStyle.Render("  Example: sgh prl --org my-org"))

			// Show any extra flags by referencing the full command
			fmt.Println(arrowStyle.Render("  Help: sgh <shortcut> --help"))
			fmt.Println()
		},
	}
}

