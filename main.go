/*
Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
*/
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/prady-lab/sgh-cli/cmd"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
)

func main() {
	ctx, err := context.Init()
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Error initializing context: %v\n", err)
		logger.Glog.Error().Err(err).Msg("Error in initializing the context")

		// Provide helpful guidance for common issues
		if strings.Contains(err.Error(), "GITHUB_TOKEN") {
			fmt.Fprintln(os.Stderr, "\n💡 Help:")
			fmt.Fprintln(os.Stderr, "   1. Create a GitHub Personal Access Token at https://github.com/settings/tokens")
			fmt.Fprintln(os.Stderr, "   2. Set the token as an environment variable:")
			fmt.Fprintln(os.Stderr, "      Windows: set GITHUB_TOKEN=your_token_here")
			fmt.Fprintln(os.Stderr, "      Linux/Mac: export GITHUB_TOKEN=your_token_here")
		}
		os.Exit(1)
	}

	rootCmd := cmd.NewRootCommand(ctx)
	if err := rootCmd.Execute(); err != nil {
		logger.Glog.Error().Err(err).Msg("Command execution failed")

		// Don't exit here - cobra already handles command errors and exit codes
		// Just log for debugging purposes
	}
}
