// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package version

import (
	"fmt"
	"runtime"

	"github.com/charmbracelet/lipgloss"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/spf13/cobra"
)

var (
	Version   = "1.0.0"
	CommitSHA = "Beta"
	BuildDate = "Beta"
)

func NewVersionCommand() *cobra.Command {
	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Display version information",
		Long: `Display detailed version information about sgh-cli.

This includes:
- Version number
- Git commit SHA
- Build date
- Go version
- Platform information`,
		Run: func(cmd *cobra.Command, args []string) {
			displayVersion()
		},
	}

	return versionCmd
}

func displayVersion() {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.Cyan)
	labelStyle := lipgloss.NewStyle().Foreground(ui.Dimmed).Width(14)
	valueStyle := lipgloss.NewStyle().Foreground(ui.White)
	accentStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.Green)

	fmt.Println()
	fmt.Println(titleStyle.Render("sgh-cli"))
	fmt.Println()

	rows := [][2]string{
		{"Version", Version},
		{"Commit SHA", CommitSHA},
		{"Build Date", BuildDate},
		{"Go Version", runtime.Version()},
		{"Platform", runtime.GOOS + "/" + runtime.GOARCH},
	}

	for _, row := range rows {
		val := valueStyle.Render(row[1])
		if row[0] == "Version" {
			val = accentStyle.Render(row[1])
		}
		fmt.Printf("  %s %s\n", labelStyle.Render(row[0]), val)
	}
	fmt.Println()
}
