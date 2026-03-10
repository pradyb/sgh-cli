// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/prady-lab/sgh-cli/pkg/config"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/ui"
)

func NewConfigCommand(ctx *context.Context) *cobra.Command {
	configCmd := &cobra.Command{
		Use:     "config <command>",
		Short:   "Manage configuration for sgh",
		Long:    `Add/Remove/List/Validate the configuration for sgh.`,
		Aliases: []string{"cfg"},
		Example: heredoc.Doc(`
			$ sgh config list
			$ sgh config validate
			$ sgh config add key value
		`),
	}

	configCmd.AddCommand(listCommand(ctx))
	configCmd.AddCommand(validateCommand(ctx))
	configCmd.AddCommand(addCommand(ctx))
	configCmd.AddCommand(setCommand(ctx))

	return configCmd
}

func listCommand(ctx *context.Context) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "Show current configuration",
		Long:    `Display the current sgh configuration, including organizations, repositories, patterns, and settings.`,
		Aliases: []string{"ls", "view"},
		Example: heredoc.Doc(`
			$ sgh config list
		`),
		Run: func(cmd *cobra.Command, args []string) {
			if ctx.JSON {
				ui.PrintJSON(ctx.Config)
				return
			}

			titleStyle := lipgloss.NewStyle().Foreground(ui.Cyan).Bold(true)
			labelStyle := lipgloss.NewStyle().Foreground(ui.White).Bold(true)
			valueStyle := lipgloss.NewStyle().Foreground(ui.Subtle)
			greenStyle := lipgloss.NewStyle().Foreground(ui.Green)

			fmt.Println()
			fmt.Println(titleStyle.Render("  SGH Configuration"))
			fmt.Println(titleStyle.Render("  ─────────────────"))
			fmt.Println()

			orgs := ctx.Config.OrganizationNames()
			if len(orgs) == 0 {
				ui.PrintNoDataMessage("No organizations configured.",
					"Hint: run 'sgh config add org <name>' to get started.")
				return
			}

			for _, orgName := range orgs {
				fmt.Printf("  %s %s\n", labelStyle.Render("Organization:"), greenStyle.Render(orgName))

				repos := ctx.Config.RepositoriesNames(orgName)
				if len(repos) > 0 {
					fmt.Printf("    %s %s\n", labelStyle.Render("Repositories:"), valueStyle.Render(fmt.Sprintf("(%d)", len(repos))))
					for _, r := range repos {
						fmt.Printf("      • %s\n", valueStyle.Render(r))
					}
				}

				incl := ctx.Config.IncludePatterns(orgName)
				if len(incl) > 0 {
					fmt.Printf("    %s %s\n", labelStyle.Render("Include Patterns:"), valueStyle.Render(strings.Join(incl, ", ")))
				}
				excl := ctx.Config.ExcludePatterns(orgName)
				if len(excl) > 0 {
					fmt.Printf("    %s %s\n", labelStyle.Render("Exclude Patterns:"), valueStyle.Render(strings.Join(excl, ", ")))
				}

				assignees := ctx.Config.PullRequestAssignees(orgName)
				if len(assignees) > 0 {
					fmt.Printf("    %s %s\n", labelStyle.Render("PR Assignees:"), valueStyle.Render(strings.Join(assignees, ", ")))
				}

				taggerName := ctx.Config.TaggerName(orgName)
				taggerEmail := ctx.Config.TaggerEmail(orgName)
				if taggerName != "" || taggerEmail != "" {
					fmt.Printf("    %s %s <%s>\n", labelStyle.Render("Tagger:"), valueStyle.Render(taggerName), valueStyle.Render(taggerEmail))
				}

				fmt.Println()
			}
		},
	}
}

func validateCommand(ctx *context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the current configuration",
		Long:  `Check the sgh configuration file for errors such as invalid patterns, duplicate orgs, and bad usernames.`,
		Example: heredoc.Doc(`
			$ sgh config validate
		`),
		Run: func(cmd *cobra.Command, args []string) {
			passStyle := lipgloss.NewStyle().Foreground(ui.Green).Bold(true)
			fmt.Println()
			fmt.Println(passStyle.Render("  ✓ Configuration is valid"))
			fmt.Println()

			orgs := ctx.Config.OrganizationNames()
			infoStyle := lipgloss.NewStyle().Foreground(ui.Subtle)
			fmt.Println(infoStyle.Render(fmt.Sprintf("  Organizations: %d", len(orgs))))
			total := 0
			for _, org := range orgs {
				repos := ctx.Config.RepositoriesNames(org)
				total += len(repos)
			}
			fmt.Println(infoStyle.Render(fmt.Sprintf("  Total Repositories: %d", total)))
			fmt.Println()
		},
	}
}

var (
	include bool
	exclude bool
)

func addCommand(ctx *context.Context) *cobra.Command {
	configAddCmd := &cobra.Command{
		Use:   "add <key> <value>",
		Short: "Add a configuration for sgh",
		Long: `Add following configurations for sgh:
New organization
New repository to organization 
include/exclude patterns to select the repository.`,
		Example: heredoc.Doc(`
			$ sgh config add org sample-org
			$ sgh config add repo sample-repo -o sample-org
			$ sgh config add pattern abc-* -o sample-org -i
			$ sgh config add pattern xyz-* -o sample-org -e
		`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("invalid arguments")
			}
			key := args[0]
			orgName, _ := cmd.Flags().GetString("org")
			if slices.Contains([]string{"repo", "repository", "pattern", "pr-assignee"}, strings.ToLower(key)) && orgName == "" {
				return fmt.Errorf("organization name is required")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			value := args[1]
			orgName, _ := cmd.Flags().GetString("org")

			switch strings.ToLower(key) {
			case "org", "organization":
				config.AddOrganization(ctx, value)
				return
			case "repo", "repository":
				config.AddRepository(ctx, orgName, value)
				return
			case "pattern":
				if include && exclude {
					logger.Glog.Error().Msgf("Both include and exclude can't be true")
					return
				}
				_, err := regexp.Compile(value)
				if err != nil {
					logger.Glog.Error().Msgf("Invalid pattern %s", value)
					return
				}
				config.AddRepositoryPattern(ctx, orgName, include, exclude, value)
				return
			case "pr-assignee":
				config.AddPullRequestAssignee(ctx, orgName, value)
				return
			default:
				logger.Glog.Error().Msgf("Invalid Key %s", key)
			}
		},
	}

	configAddCmd.Flags().BoolVarP(&include, "include", "i", false, "The `regex pattern` to select the repositories to include for processing")
	configAddCmd.Flags().BoolVarP(&exclude, "exclude", "e", false, "The `regex pattern` if you want to exclude some repositories from processing")
	configAddCmd.MarkFlagsMutuallyExclusive("include", "exclude")
	return configAddCmd
}

func setCommand(ctx *context.Context) *cobra.Command {
	configSetCmd := &cobra.Command{
		Use:   "set",
		Short: "Set a configuration for sgh",
		Long:  `Set attribute values`,
		Example: heredoc.Doc(`
			$ sgh config set tagger-name "John Doe"
			$ sgh config set tagger-email "john.doe@sample.com"
		`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("invalid arguments")
			}
			key := args[0]
			orgName, _ := cmd.Flags().GetString("org")
			if slices.Contains([]string{"tagger-name", "tagger-email"}, strings.ToLower(key)) && orgName == "" {
				return fmt.Errorf("organization name is required")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			value := args[1]
			orgName, _ := cmd.Flags().GetString("org")

			switch strings.ToLower(key) {
			case "tagger-name":
				config.SetTaggerName(ctx, orgName, value)
				return
			case "tagger-email":
				config.SetTaggerEmail(ctx, orgName, value)
				return
			default:
				logger.Glog.Error().Msgf("Invalid Key %s", key)
			}
		},
	}
	return configSetCmd
}
