package health

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
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
	fmt.Println("🔍 Running sgh-cli health check...")
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
	for _, check := range checks {
		fmt.Printf("  %s... ", check.name)
		if err := check.fn(ctx); err != nil {
			fmt.Printf("❌ FAILED\n     Error: %v\n", err)
			allPassed = false
		} else {
			fmt.Println("✅ PASSED")
		}
	}

	fmt.Println()
	if allPassed {
		fmt.Println("🎉 All health checks passed! sgh-cli is ready to use.")
	} else {
		fmt.Println("⚠️  Some health checks failed. Please review the errors above.")
		fmt.Println("💡 Run with --verbose for more detailed information.")
	}
}

func checkGitHubAPIConnectivity(ctx *context.Context) error {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.github.com/zen")
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
	// Test authentication by making a simple API call
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Get token from environment
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return fmt.Errorf("GITHUB_TOKEN environment variable not set")
	}

	req.Header.Set("Authorization", fmt.Sprintf("token %s", token))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
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
		return fmt.Errorf("no rate limit information available")
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
