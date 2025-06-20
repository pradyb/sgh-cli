package apperrors

import (
	"fmt"
	"net/http"
)

// GitHubError represents errors from GitHub API
type GitHubError struct {
	StatusCode int
	Message    string
	URL        string
}

func (e *GitHubError) Error() string {
	return fmt.Sprintf("GitHub API error (%d): %s", e.StatusCode, e.Message)
}

func (e *GitHubError) IsRateLimit() bool {
	return e.StatusCode == http.StatusForbidden &&
		(e.Message == "API rate limit exceeded" || e.StatusCode == 403)
}

func (e *GitHubError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

func (e *GitHubError) IsUnauthorized() bool {
	return e.StatusCode == http.StatusUnauthorized
}

// ConfigError represents configuration-related errors
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("configuration error for %s: %s", e.Field, e.Message)
}

// ValidationError represents input validation errors
type ValidationError struct {
	Field   string
	Value   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for %s='%s': %s", e.Field, e.Value, e.Message)
}
