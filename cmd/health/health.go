package health

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/ui"
	"github.com/spf13/cobra"
)

func NewHealthCommand(ctx *context.Context) *cobra.Command {
	healthCmd := &cobra.Command{
		Use:   "health",
		Short: "🔍 Check the health and connectivity of sgh-cli",
		Long: `Check the health and connectivity of sgh-cli components:

• GitHub API connectivity
• Authentication status
• Rate limit status
• Configuration validity
• Network connectivity

This command helps diagnose issues with the CLI setup.`,
		Run: func(cmd *cobra.Command, args []string) {
			runHealthCheck(ctx)
		},
	}

	return healthCmd
}

func runHealthCheck(ctx *context.Context) {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.Cyan)
	labelStyle := lipgloss.NewStyle().Foreground(ui.White).Width(32)
	passStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.Green)
	failStyle := lipgloss.NewStyle().Bold(true).Foreground(ui.Red)
	errDetailStyle := lipgloss.NewStyle().Foreground(ui.Dimmed).Italic(true).PaddingLeft(6)
	summaryOk := lipgloss.NewStyle().Bold(true).Foreground(ui.Green)
	summaryFail := lipgloss.NewStyle().Bold(true).Foreground(ui.Yellow)
	hintStyle := lipgloss.NewStyle().Foreground(ui.Dimmed).Italic(true)

	fmt.Println()
	fmt.Println(titleStyle.Render("  sgh-cli health check"))
	fmt.Println()

	checks := []struct {
		name string
		fn   func(*context.Context) error
	}{
		{"GitHub API Connectivity", checkGitHubAPIConnectivity},
		{"Authentication", checkAuthentication},
		{"Rate Limit Status", checkRateLimitStatus},
		{"Configuration", checkConfiguration},
		{"Network Connectivity", checkNetworkConnectivity},
	}

	allPassed := true
	passCount := 0
	for _, check := range checks {
		err := check.fn(ctx)
		if err != nil {
			fmt.Printf("  %s %s\n", failStyle.Render("✗"), labelStyle.Render(check.name))
			fmt.Println(errDetailStyle.Render(err.Error()))
			allPassed = false
		} else {
			fmt.Printf("  %s %s\n", passStyle.Render("✓"), labelStyle.Render(check.name))
			passCount++
		}
	}

	fmt.Println()
	if allPassed {
		fmt.Printf("  %s\n", summaryOk.Render(fmt.Sprintf("All %d checks passed — sgh-cli is ready to use.", passCount)))
	} else {
		fmt.Printf("  %s\n", summaryFail.Render(fmt.Sprintf("%d/%d checks passed. Review the errors above.", passCount, len(checks))))
		fmt.Printf("  %s\n", hintStyle.Render("Run with --verbose for more detailed information."))
	}
	fmt.Println()
}

func checkGitHubAPIConnectivity(ctx *context.Context) error {
	req, err := http.NewRequest("GET", "https://api.github.com/zen", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := ctx.HttpClient.Send(req)
	if err != nil {
		return fmt.Errorf("cannot reach GitHub API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	return nil
}

func checkAuthentication(ctx *context.Context) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN environment variable not set")
	}

	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := ctx.HttpClient.Send(req)
	if err != nil {
		return fmt.Errorf("authentication request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("invalid or expired token")
	} else if resp.StatusCode != 200 {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func checkRateLimitStatus(ctx *context.Context) error {
	status := ctx.HttpClient.GetRateLimitStatus()
	if len(status) == 0 {
		// logger.Glog.Info().Msg("No rate limit information available (skipping rate limit check)")
		return nil
	}

	// Check if we're close to rate limits
	for resource, info := range status {
		if info.Remaining < 100 {
			logger.Glog.Warn().
				Str("resource", resource).
				Int("remaining", info.Remaining).
				Msg("Rate limit is low")
		}
	}

	return nil
}

func checkConfiguration(ctx *context.Context) error {
	if ctx.Config == nil {
		return fmt.Errorf("configuration is nil")
	}

	// Check if we have any organizations configured
	if len(ctx.Config.Organizations) == 0 {
		logger.Glog.Info().Msg("No organizations configured (this is normal for new installations)")
	}

	return nil
}

func checkNetworkConnectivity(ctx *context.Context) error {
	// Test basic internet connectivity
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://www.google.com")
	if err != nil {
		return fmt.Errorf("no internet connectivity: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
