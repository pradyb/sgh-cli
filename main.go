// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/prady-lab/sgh-cli/cmd"
	appcontext "github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
)

func main() {
	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals with proper cleanup
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Use a separate goroutine with done channel for proper cleanup
	done := make(chan bool, 1)
	go func() {
		defer func() {
			signal.Stop(sigChan) // Stop signal notifications
			close(sigChan)       // Close the channel
			done <- true         // Signal completion
		}()

		<-sigChan
		logger.Flog.Info().Msg("Received interrupt signal, shutting down gracefully...")
		cancel()

		select {
		case <-time.After(2 * time.Second):
			logger.Flog.Warn().Msg("Graceful shutdown timeout, forcing exit")
		case <-ctx.Done():
			logger.Flog.Info().Msg("Graceful shutdown completed")
		}
		os.Exit(0)
	}()

	appCtx, err := appcontext.Init()
	if err != nil {
		handleInitializationError(err)
		// Ensure signal handler cleanup before exit
		cancel()
		select {
		case <-done:
		case <-time.After(100 * time.Millisecond):
		}
		os.Exit(1)
	}

	// Create root command
	rootCmd := cmd.NewRootCommand(appCtx)

	// Execute with context for graceful shutdown
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		logger.Flog.Error().Err(err).Msg("Command execution failed")
		// Don't exit here - cobra already handles command errors and exit codes
		// Just log for debugging purposes
	}
}

// handleInitializationError provides detailed error messages and guidance
func handleInitializationError(err error) {
	fmt.Fprintf(os.Stderr, "❌ Error initializing sgh-cli: %v\n", err)
	logger.Glog.Error().Err(err).Msg("Error in initializing the context")

	// Provide specific guidance based on error type
	switch {
	case strings.Contains(err.Error(), "GITHUB_TOKEN"):
		printGitHubTokenHelp()
	case strings.Contains(err.Error(), "invalid GITHUB_TOKEN"):
		printTokenValidationHelp()
	case strings.Contains(err.Error(), "failed to initialize config"):
		printConfigHelp()
	default:
		printGeneralHelp()
	}
}

func printGitHubTokenHelp() {
	fmt.Fprintln(os.Stderr, "\n💡 GitHub Token Setup:")
	fmt.Fprintln(os.Stderr, "   1. Create a GitHub Personal Access Token:")
	fmt.Fprintln(os.Stderr, "      • Go to https://github.com/settings/tokens")
	fmt.Fprintln(os.Stderr, "      • Click 'Generate new token (classic)'")
	fmt.Fprintln(os.Stderr, "      • Select scopes: repo, admin:org")
	fmt.Fprintln(os.Stderr, "   2. Set the token as an environment variable:")
	fmt.Fprintln(os.Stderr, "      Linux/Mac: export GITHUB_TOKEN=your_token_here")
	fmt.Fprintln(os.Stderr, "      Windows: set GITHUB_TOKEN=your_token_here")
	fmt.Fprintln(os.Stderr, "   3. For persistence, add to your shell profile")
}

func printTokenValidationHelp() {
	fmt.Fprintln(os.Stderr, "\n🔍 Token Validation Issues:")
	fmt.Fprintln(os.Stderr, "   • Token must be at least 20 characters long")
	fmt.Fprintln(os.Stderr, "   • Token must start with: ghp_, gho_, ghu_, ghs_, ghr_, or github_pat_")
	fmt.Fprintln(os.Stderr, "   • Token cannot contain spaces")
	fmt.Fprintln(os.Stderr, "   • Test tokens (starting with 'ghp_test_') are not allowed")
	fmt.Fprintln(os.Stderr, "   • Verify your token is valid at https://github.com/settings/tokens")
}

func printConfigHelp() {
	fmt.Fprintln(os.Stderr, "\n⚙️ Configuration Issues:")
	fmt.Fprintln(os.Stderr, "   • Check if config file exists and is valid JSON")
	fmt.Fprintln(os.Stderr, "   • Config locations:")
	fmt.Fprintln(os.Stderr, "     Linux/Mac: ~/.config/sgh/sgh.json")
	fmt.Fprintln(os.Stderr, "     Windows: ~/sgh.json")
	fmt.Fprintln(os.Stderr, "   • Try running: sgh config list")
}

func printGeneralHelp() {
	fmt.Fprintln(os.Stderr, "\n🔧 General Troubleshooting:")
	fmt.Fprintln(os.Stderr, "   • Check your internet connection")
	fmt.Fprintln(os.Stderr, "   • Verify GitHub API is accessible")
	fmt.Fprintln(os.Stderr, "   • Run with --verbose flag for detailed logs")
	fmt.Fprintln(os.Stderr, "   • Check the documentation: https://github.com/prady-lab/sgh-cli")
}
