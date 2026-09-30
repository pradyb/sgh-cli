// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package config

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
	"golang.org/x/term"

	internalconfig "github.com/pradyb/sgh-cli/internal/config"
	"github.com/pradyb/sgh-cli/pkg/config"
	"github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/logger"
	"github.com/pradyb/sgh-cli/pkg/ui"
)

// isStdinTerminal is a var so tests can force the non-interactive path deterministically
// without needing a real terminal.
var isStdinTerminal = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

// readToken reads a token value: masked and interactive if stdin is a real terminal (via
// term.ReadPassword, so it never lands in shell history), otherwise a single trimmed
// line from in. The non-terminal path also makes scripted/CI usage work:
// echo "$TOKEN" | sgh config set token --org my-org.
func readToken(in io.Reader) (string, error) {
	if isStdinTerminal() {
		fmt.Fprint(os.Stderr, "  Token: ")
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		return strings.TrimSpace(string(b)), err
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func NewConfigCommand(ctx *context.Context) *cobra.Command {
	configCmd := &cobra.Command{
		Use:     "config <command>",
		Short:   "Manage configuration for sgh",
		Long:    `Add, remove, set, list, and validate the sgh configuration.`,
		Aliases: []string{"cfg"},
		Example: heredoc.Doc(`
			$ sgh config list
			$ sgh config validate
			$ sgh config add org my-org
			$ sgh config remove org my-org
			$ sgh config set tagger-name "Jane Doe" --org my-org
		`),
	}

	configCmd.AddCommand(listCommand(ctx))
	configCmd.AddCommand(validateCommand(ctx))
	configCmd.AddCommand(addCommand(ctx))
	configCmd.AddCommand(removeCommand(ctx))
	configCmd.AddCommand(setCommand(ctx))
	configCmd.AddCommand(resetCommand(ctx))

	return configCmd
}

func listCommand(ctx *context.Context) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "Show current configuration",
		Long:    `Display the current sgh configuration, including organizations, repositories, patterns, and settings.`,
		Aliases: []string{"ls", "view", "show"},
		Example: heredoc.Doc(`
			$ sgh config list
		`),
		Run: func(cmd *cobra.Command, args []string) {
			if ctx.JSON {
				ui.PrintJSON(ctx.Config)
				return
			}

			orgs := ctx.Config.OrganizationNames()
			if len(orgs) == 0 {
				ui.PrintNoDataMessage("No organizations configured.",
					"Hint: run 'sgh config add org <name>' to get started.")
				return
			}

			printConfigTable(ctx, orgs)
		},
	}
}

func validateCommand(ctx *context.Context) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the current configuration",
		Long: heredoc.Doc(`
			Check the sgh configuration file for errors:
			  • Invalid or overly-broad regex patterns (include/exclude)
			  • Duplicate organization names
			  • Invalid organization/repo names
			  • Invalid tagger email addresses
			  • Invalid user list entries

			Exits with code 1 if any errors are found.
		`),
		Example: heredoc.Doc(`
			$ sgh config validate
		`),
		Run: func(cmd *cobra.Command, args []string) {
			passStyle := lipgloss.NewStyle().Foreground(ui.Green).Bold(true)
			failStyle := lipgloss.NewStyle().Foreground(ui.Red).Bold(true)
			orgStyle := lipgloss.NewStyle().Foreground(ui.Cyan).Bold(true)
			labelStyle := lipgloss.NewStyle().Foreground(ui.Subtle)
			dimStyle := lipgloss.NewStyle().Foreground(ui.Dimmed).Italic(true)
			includeStyle := lipgloss.NewStyle().Foreground(ui.Green)
			excludeStyle := lipgloss.NewStyle().Foreground(ui.Red)
			warnPatStyle := lipgloss.NewStyle().Foreground(ui.Yellow)

			fmt.Println()

			// Full structural validation (duplicate orgs, invalid names, emails, etc.)
			if err := ctx.Config.Validate(); err != nil {
				fmt.Println(failStyle.Render("  ✗ Configuration has errors:"))
				fmt.Println(failStyle.Render("    " + err.Error()))
				fmt.Println()
				ctx.HasError = true
				return
			}

			orgs := ctx.Config.OrganizationNames()
			totalRepos := 0
			totalInclude := 0
			totalExclude := 0
			// Validate() already checked all patterns structurally; here we re-check
			// to produce a human-readable per-pattern report with warnings.
			patternWarnings := 0

			for _, orgName := range orgs {
				repos := ctx.Config.RepositoriesNames(orgName)
				incl := ctx.Config.IncludePatterns(orgName)
				excl := ctx.Config.ExcludePatterns(orgName)
				totalRepos += len(repos)
				totalInclude += len(incl)
				totalExclude += len(excl)

				fmt.Printf("  %s  %s\n",
					orgStyle.Render(orgName),
					dimStyle.Render(fmt.Sprintf("(%d repos in fuzzy dict)", len(repos))))

				if len(incl) == 0 && len(excl) == 0 {
					fmt.Println(labelStyle.Render("    patterns: ") + dimStyle.Render("none — all repos selected"))
				}
				for _, p := range incl {
					icon := includeStyle.Render("✓ include")
					status := ""
					if err := internalconfig.ValidatePattern(p); err != nil {
						icon = warnPatStyle.Render("⚠ include")
						status = warnPatStyle.Render("  ← " + err.Error())
						patternWarnings++
					}
					fmt.Printf("    %s  %s%s\n", icon, includeStyle.Render(p), status)
				}
				for _, p := range excl {
					icon := excludeStyle.Render("✗ exclude")
					status := ""
					if err := internalconfig.ValidatePattern(p); err != nil {
						icon = warnPatStyle.Render("⚠ exclude")
						status = warnPatStyle.Render("  ← " + err.Error())
						patternWarnings++
					}
					fmt.Printf("    %s  %s%s\n", icon, excludeStyle.Render(p), status)
				}
			}

			fmt.Println()
			fmt.Printf("  %s  %s  %s\n",
				labelStyle.Render(fmt.Sprintf("Orgs: %d", len(orgs))),
				labelStyle.Render(fmt.Sprintf("Repos: %d", totalRepos)),
				labelStyle.Render(fmt.Sprintf("Patterns: %d include, %d exclude", totalInclude, totalExclude)))
			fmt.Println()

			if patternWarnings > 0 {
				fmt.Println(warnPatStyle.Render(fmt.Sprintf("  ⚠ %d pattern(s) have warnings", patternWarnings)))
				fmt.Println()
				ctx.HasError = true
				return
			}

			fmt.Println(passStyle.Render("  ✓ Configuration is valid"))
			fmt.Println()
		},
	}
}

func addCommand(ctx *context.Context) *cobra.Command {
	// Flags are local — avoids shared state if the command is invoked multiple times.
	var include, exclude bool

	configAddCmd := &cobra.Command{
		Use:   "add <key> <value>",
		Short: "Add a configuration value",
		Long: heredoc.Doc(`
			Add a value to the sgh configuration.

			Valid keys:
			  org          Add a new organization (value = org name)
			  repo         Add a repository to an org  (requires --org)
			  pattern      Add an include/exclude repo filter pattern  (requires --org and --include or --exclude)
			  pr-assignee  Add a default PR assignee for an org  (requires --org)
		`),
		Example: heredoc.Doc(`
			$ sgh config add org my-org
			$ sgh config add repo my-repo --org my-org
			$ sgh config add pattern "^api-" --org my-org --include
			$ sgh config add pattern "-legacy$" --org my-org --exclude
			$ sgh config add pr-assignee john-doe --org my-org
		`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("expected exactly 2 arguments: <key> <value>, got %d", len(args))
			}
			key := strings.ToLower(args[0])
			orgName, _ := cmd.Flags().GetString("org")
			if key == "repo" || key == "repository" || key == "pattern" || key == "pr-assignee" {
				if orgName == "" {
					return fmt.Errorf("key %q requires --org <organization>", args[0])
				}
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
			case "repo", "repository":
				config.AddRepository(ctx, orgName, value)
			case "pattern":
				if !include && !exclude {
					ui.PrintCLIError(
						"Must specify --include (-i) or --exclude (-e) when adding a pattern.",
						"Example: sgh config add pattern \"^api-\" --org my-org --include",
					)
					return
				}
				if err := internalconfig.ValidatePattern(value); err != nil {
					ui.PrintCLIError(
						fmt.Sprintf("Invalid pattern: %s", err),
						"Patterns must be valid Go regular expressions.",
						"Use anchors for precision, e.g. \"^api-\" or \"-legacy$\"",
						"Avoid catch-all patterns like \".*\" or \".+\" that match everything.",
					)
					return
				}
				kind := "include"
				if exclude {
					kind = "exclude"
				}
				logger.Flog.Info().Str("pattern", value).Str("kind", kind).Str("org", orgName).Msg("Adding repository pattern")
				config.AddRepositoryPattern(ctx, orgName, include, exclude, value)
			case "pr-assignee":
				config.AddPullRequestAssignee(ctx, orgName, value)
			default:
				ui.PrintCLIError(
					fmt.Sprintf("Unknown key %q", key),
					"Valid keys: org, repo, pattern, pr-assignee",
				)
			}
		},
	}

	configAddCmd.Flags().BoolVarP(&include, "include", "i", false, "Pattern applies to repositories to include")
	configAddCmd.Flags().BoolVarP(&exclude, "exclude", "e", false, "Pattern applies to repositories to exclude")
	configAddCmd.MarkFlagsMutuallyExclusive("include", "exclude")
	return configAddCmd
}

func removeCommand(ctx *context.Context) *cobra.Command {
	var include, exclude bool

	configRemoveCmd := &cobra.Command{
		Use:     "remove <key> <value>",
		Short:   "Remove a configuration value",
		Aliases: []string{"rm", "delete"},
		Long: heredoc.Doc(`
			Remove a value from the sgh configuration.

			Valid keys:
			  org          Remove an organization and all its data (value = org name)
			  repo         Remove a repository from an org  (requires --org)
			  pattern      Remove an include/exclude repo filter pattern  (requires --org and --include or --exclude)
			  pr-assignee  Remove a default PR assignee from an org  (requires --org)
			  token        Remove the token for an org, from the OS keyring if that's where it lives  (requires --org)
		`),
		Example: heredoc.Doc(`
			$ sgh config remove org my-org
			$ sgh config remove repo my-repo --org my-org
			$ sgh config remove pattern "^api-" --org my-org --include
			$ sgh config remove pattern "-legacy$" --org my-org --exclude
			$ sgh config remove pr-assignee john-doe --org my-org
			$ sgh config remove token --org my-org
		`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("expected a key, got no arguments")
			}
			key := strings.ToLower(args[0])
			orgName, _ := cmd.Flags().GetString("org")
			if key == "token" {
				if len(args) != 1 {
					return fmt.Errorf("key %q takes no value — run: sgh config remove token --org <organization>", args[0])
				}
				if orgName == "" {
					return fmt.Errorf("key %q requires --org <organization>", args[0])
				}
				return nil
			}
			if len(args) != 2 {
				return fmt.Errorf("expected exactly 2 arguments: <key> <value>, got %d", len(args))
			}
			if key == "repo" || key == "repository" || key == "pattern" || key == "pr-assignee" {
				if orgName == "" {
					return fmt.Errorf("key %q requires --org <organization>", args[0])
				}
			}
			if key == "pattern" {
				if !include && !exclude {
					return fmt.Errorf("removing a pattern requires --include (-i) or --exclude (-e)")
				}
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			key := strings.ToLower(args[0])
			orgName, _ := cmd.Flags().GetString("org")

			if key == "token" {
				removed, err := config.RemoveToken(ctx, orgName)
				if err != nil {
					ui.PrintCLIError(fmt.Sprintf("Failed to remove token: %s", err), "")
					return
				}
				if removed {
					fmt.Println(lipgloss.NewStyle().Foreground(ui.Green).Render(fmt.Sprintf("  Token removed for %s", orgName)))
				} else {
					fmt.Println(lipgloss.NewStyle().Foreground(ui.Yellow).Render(fmt.Sprintf("  No token configured for %s", orgName)))
				}
				return
			}

			value := args[1]
			switch key {
			case "org", "organization":
				config.RemoveOrganization(ctx, value)
			case "repo", "repository":
				config.RemoveRepository(ctx, orgName, value)
			case "pattern":
				config.RemoveRepositoryPattern(ctx, orgName, include, value)
			case "pr-assignee":
				config.RemovePullRequestAssignee(ctx, orgName, value)
			default:
				ui.PrintCLIError(
					fmt.Sprintf("Unknown key %q", key),
					"Valid keys: org, repo, pattern, pr-assignee, token",
				)
			}
		},
	}

	configRemoveCmd.Flags().BoolVarP(&include, "include", "i", false, "Pattern is in the include list")
	configRemoveCmd.Flags().BoolVarP(&exclude, "exclude", "e", false, "Pattern is in the exclude list")
	configRemoveCmd.MarkFlagsMutuallyExclusive("include", "exclude")
	return configRemoveCmd
}

func setCommand(ctx *context.Context) *cobra.Command {
	configSetCmd := &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: heredoc.Doc(`
			Set a scalar configuration value for an organization.

			Valid keys:
			  token          Per-org GitHub token (fine-grained PAT)  (requires --org)
			  owner-type     Owner type: "Organization" or "User"  (requires --org)
			  tagger-name    Git commit tagger display name  (requires --org)
			  tagger-email   Git commit tagger email address  (requires --org)
		`),
		Example: heredoc.Doc(`
			$ sgh config set token --org my-org
			$ sgh config set owner-type User --org pradyb
			$ sgh config set tagger-name "Jane Doe" --org my-org
			$ sgh config set tagger-email "jane@example.com" --org my-org
		`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("expected a key, got no arguments")
			}
			key := strings.ToLower(args[0])
			orgName, _ := cmd.Flags().GetString("org")
			if key == "token" {
				if len(args) != 1 {
					return fmt.Errorf("the token value is entered interactively, not as an argument — run: sgh config set token --org <organization>")
				}
			} else if len(args) != 2 {
				return fmt.Errorf("expected exactly 2 arguments: <key> <value>, got %d", len(args))
			}
			if (key == "token" || key == "owner-type" || key == "tagger-name" || key == "tagger-email") && orgName == "" {
				return fmt.Errorf("key %q requires --org <organization>", args[0])
			}
			if key == "owner-type" {
				v := args[1]
				if v != "Organization" && v != "User" {
					return fmt.Errorf("owner-type must be \"Organization\" or \"User\", got %q", v)
				}
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			key := strings.ToLower(args[0])
			orgName, _ := cmd.Flags().GetString("org")

			if key == "token" {
				value, err := readToken(cmd.InOrStdin())
				if err != nil {
					ui.PrintCLIError(fmt.Sprintf("Failed to read token: %s", err), "")
					return
				}
				if err := context.ValidateGitHubToken(value); err != nil {
					ui.PrintCLIError(fmt.Sprintf("Invalid token: %s", err), "Expected a GitHub PAT (e.g. github_pat_... or ghp_...)")
					return
				}
				if config.SetToken(ctx, orgName, value) {
					fmt.Println(lipgloss.NewStyle().Foreground(ui.Green).Render(fmt.Sprintf("  Token for %s stored in the OS keyring", orgName)))
				} else {
					warnStyle := lipgloss.NewStyle().Foreground(ui.Yellow)
					fmt.Println(warnStyle.Render("  ⚠ OS keyring unavailable — token stored in plain text in the config file."))
					fmt.Println(warnStyle.Render("    Keep the config file out of version control."))
					fmt.Println(lipgloss.NewStyle().Foreground(ui.Green).Render(fmt.Sprintf("  Token set for %s", orgName)))
				}
				return
			}

			value := args[1]
			switch key {
			case "owner-type":
				config.SetOwnerType(ctx, orgName, value)
				fmt.Println(lipgloss.NewStyle().Foreground(ui.Green).Render(fmt.Sprintf("  Owner type for %s set to %q", orgName, value)))
			case "tagger-name":
				config.SetTaggerName(ctx, orgName, value)
			case "tagger-email":
				config.SetTaggerEmail(ctx, orgName, value)
			default:
				ui.PrintCLIError(
					fmt.Sprintf("Unknown key %q", key),
					"Valid keys: token, owner-type, tagger-name, tagger-email",
				)
				logger.Flog.Warn().Str("key", key).Msg("Unknown config set key")
			}
		},
	}
	return configSetCmd
}

func resetCommand(ctx *context.Context) *cobra.Command {
	var orgName string
	var force bool

	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset (clear) configuration",
		Long: heredoc.Doc(`
			Reset sgh configuration.

			Without --org: removes all organizations and their data from the config file.
			With --org:    removes only that organization and all its repos, patterns, and settings.

			Use --yes to skip the confirmation prompt.
		`),
		Example: heredoc.Doc(`
			$ sgh config reset --yes
			$ sgh config reset --org my-org
			$ sgh config reset --org my-org --yes
		`),
		Run: func(cmd *cobra.Command, args []string) {
			passStyle := lipgloss.NewStyle().Foreground(ui.Green).Bold(true)
			warnStyle := lipgloss.NewStyle().Foreground(ui.Yellow).Bold(true)
			if v, _ := cmd.Flags().GetBool("force"); v {
				force = true
			}

			if orgName != "" {
				if !force {
					fmt.Printf("\n  %s\n\n", warnStyle.Render(fmt.Sprintf("This will remove organization %q and all its data.", orgName)))
					fmt.Print("  Type the organization name to confirm: ")
					var confirm string
					fmt.Scanln(&confirm)
					if confirm != orgName {
						fmt.Println(warnStyle.Render("  Aborted."))
						return
					}
				}
				if !ctx.Config.RemoveOrganization(orgName) {
					ui.PrintCLIError(fmt.Sprintf("Organization %q not found in config", orgName), "")
					return
				}
				if err := ctx.Config.Save(); err != nil {
					ui.PrintCLIError("Failed to save config", err.Error())
					return
				}
				fmt.Println(passStyle.Render(fmt.Sprintf("  Organization %q removed from config.", orgName)))
				return
			}

			if !force {
				orgs := ctx.Config.OrganizationNames()
				fmt.Printf("\n  %s\n", warnStyle.Render(fmt.Sprintf("This will remove ALL %d organization(s) and their data.", len(orgs))))
				fmt.Print("  Type 'yes' to confirm: ")
				var confirm string
				fmt.Scanln(&confirm)
				if confirm != "yes" {
					fmt.Println(warnStyle.Render("  Aborted."))
					return
				}
			}

			ctx.Config.Organizations = nil
			if err := ctx.Config.Save(); err != nil {
				ui.PrintCLIError("Failed to save config", err.Error())
				return
			}
			fmt.Println(passStyle.Render("  Configuration reset successfully."))
		},
	}

	resetCmd.Flags().StringVarP(&orgName, "org", "o", "", "reset only this organization (omit to reset all)")
	resetCmd.Flags().BoolVarP(&force, "yes", "y", false, "skip the confirmation prompt")
	resetCmd.Flags().Bool("force", false, "alias for --yes (deprecated)")
	resetCmd.Flags().MarkHidden("force")
	return resetCmd
}

func printConfigTable(ctx *context.Context, orgs []string) {
	headerStyle := lipgloss.NewStyle().Padding(0, 1).Foreground(ui.Cyan).Bold(true).Align(lipgloss.Center)
	cellStyle := lipgloss.NewStyle().Padding(0, 1)
	orgStyle := cellStyle.Foreground(ui.Green).Bold(true)
	includeStyle := cellStyle.Foreground(ui.Green)
	excludeStyle := cellStyle.Foreground(ui.Red)
	mutedStyle := cellStyle.Foreground(ui.Subtle)
	dimStyle := cellStyle.Foreground(ui.Dimmed).Italic(true)
	warnStyle := cellStyle.Foreground(ui.Yellow)
	borderStyle := lipgloss.NewStyle().Foreground(ui.Dimmed)

	bullet := func(items []string, style lipgloss.Style) string {
		if len(items) == 0 {
			return dimStyle.Render("—")
		}
		lines := make([]string, len(items))
		for i, v := range items {
			lines[i] = style.Render("• " + v)
		}
		return strings.Join(lines, "\n")
	}

	rows := make([][]string, 0, len(orgs))
	for _, orgName := range orgs {
		incl := ctx.Config.IncludePatterns(orgName)
		excl := ctx.Config.ExcludePatterns(orgName)
		repos := ctx.Config.RepositoriesNames(orgName)
		assignees := ctx.Config.PullRequestAssignees(orgName)

		taggerName := ctx.Config.TaggerName(orgName)
		taggerEmail := ctx.Config.TaggerEmail(orgName)
		tagger := ""
		if taggerName != "" || taggerEmail != "" {
			tagger = taggerName + "\n<" + taggerEmail + ">"
		}

		repoCell := bullet(repos, mutedStyle)
		if len(repos) > 0 {
			repoCell += "\n" + dimStyle.Render(fmt.Sprintf("(%d, fuzzy-match dict)", len(repos)))
		}

		// Presence and source only — the token value is never displayed.
		tokenCell := dimStyle.Render("—")
		switch ctx.Config.TokenSourceForOwner(orgName) {
		case internalconfig.TokenSourceKeyring:
			tokenCell = includeStyle.Render("✓ keyring")
		case "plaintext":
			tokenCell = warnStyle.Render("✓ plaintext")
		}
		ownerTypeCell := dimStyle.Render("—")
		if ot := ctx.Config.OwnerTypeFor(orgName); ot != "" {
			ownerTypeCell = mutedStyle.Render(ot)
		}

		rows = append(rows, []string{
			orgStyle.Render(orgName),
			bullet(incl, includeStyle),
			bullet(excl, excludeStyle),
			repoCell,
			bullet(assignees, mutedStyle),
			mutedStyle.Render(tagger),
			tokenCell,
			ownerTypeCell,
		})
	}

	// Footer row
	rows = append(rows, []string{
		lipgloss.NewStyle().Padding(0, 1).Foreground(ui.Cyan).Bold(true).Align(lipgloss.Center).Render("Total"),
		lipgloss.NewStyle().Padding(0, 1).Foreground(ui.Cyan).Bold(true).Align(lipgloss.Center).Render(fmt.Sprintf("%d orgs", len(orgs))),
		"", "", "", "", "", "",
	})

	fmt.Println()
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(borderStyle).
		BorderRow(true).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == -1 {
				return headerStyle
			}
			return lipgloss.NewStyle()
		}).
		Headers("Organization", "Include Patterns", "Exclude Patterns", "Repositories (fuzzy dict)", "PR Assignees", "Tagger", "Token", "Owner Type").
		Rows(rows...)

	fmt.Println(t)

	// Show the config file path so the user knows where to find/edit it directly.
	filePath := config.ConfigFilePath()
	fmt.Println(dimStyle.Render("  Config file: " + filePath))
	fmt.Println(dimStyle.Render("  Include/Exclude patterns are Go regex. Exclude always wins over Include."))
	fmt.Println(dimStyle.Render("  Repositories list is auto-populated for fuzzy matching with the -r flag."))
	fmt.Println()
}
