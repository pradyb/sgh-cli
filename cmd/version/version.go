package version

import (
	"fmt"
	"runtime"

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
		Short: "📋 Display version information",
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
	fmt.Printf("🚀 sgh-cli version information:\n\n")
	fmt.Printf("Version:     %s\n", Version)
	fmt.Printf("Commit SHA:  %s\n", CommitSHA)
	fmt.Printf("Build Date:  %s\n", BuildDate)
	fmt.Printf("Go Version:  %s\n", runtime.Version())
	fmt.Printf("Platform:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Architecture: %s\n", runtime.GOARCH)
}
