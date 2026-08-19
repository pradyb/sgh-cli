// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package validation

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/pradyb/sgh-cli/pkg/apperrors"
)

// GitHubValidator provides validation for GitHub-specific inputs
type GitHubValidator struct {
	orgNamePattern  *regexp.Regexp
	repoNamePattern *regexp.Regexp
	usernamePattern *regexp.Regexp
}

// NewGitHubValidator creates a new GitHub validator
func NewGitHubValidator() *GitHubValidator {
	return &GitHubValidator{
		orgNamePattern:  regexp.MustCompile(`^[a-zA-Z0-9\-._]+$`),
		repoNamePattern: regexp.MustCompile(`^[a-zA-Z0-9\-._]+$`),
		usernamePattern: regexp.MustCompile(`^[a-zA-Z0-9\-]+$`),
	}
}

// ValidateOrganizationName validates GitHub organization name
func (v *GitHubValidator) ValidateOrganizationName(orgName string) error {
	if orgName == "" {
		return &apperrors.ValidationError{
			Field:   "organization_name",
			Value:   orgName,
			Message: "organization name cannot be empty",
		}
	}

	if len(orgName) > 39 {
		return &apperrors.ValidationError{
			Field:   "organization_name",
			Value:   orgName,
			Message: "organization name cannot exceed 39 characters",
		}
	}

	if !v.orgNamePattern.MatchString(orgName) {
		return &apperrors.ValidationError{
			Field:   "organization_name",
			Value:   orgName,
			Message: "organization name contains invalid characters",
		}
	}

	if strings.HasPrefix(orgName, "-") || strings.HasSuffix(orgName, "-") {
		return &apperrors.ValidationError{
			Field:   "organization_name",
			Value:   orgName,
			Message: "organization name cannot start or end with hyphen",
		}
	}

	return nil
}

// ValidateRepositoryName validates GitHub repository name
func (v *GitHubValidator) ValidateRepositoryName(repoName string) error {
	if repoName == "" {
		return &apperrors.ValidationError{
			Field:   "repository_name",
			Value:   repoName,
			Message: "repository name cannot be empty",
		}
	}

	if len(repoName) > 100 {
		return &apperrors.ValidationError{
			Field:   "repository_name",
			Value:   repoName,
			Message: "repository name cannot exceed 100 characters",
		}
	}

	if !v.repoNamePattern.MatchString(repoName) {
		return &apperrors.ValidationError{
			Field:   "repository_name",
			Value:   repoName,
			Message: "repository name contains invalid characters",
		}
	}

	// Reserved names
	reservedNames := []string{".", "..", ".git", ".github"}
	for _, reserved := range reservedNames {
		if strings.EqualFold(repoName, reserved) {
			return &apperrors.ValidationError{
				Field:   "repository_name",
				Value:   repoName,
				Message: fmt.Sprintf("'%s' is a reserved repository name", reserved),
			}
		}
	}

	return nil
}

// ValidateUsername validates GitHub username
func (v *GitHubValidator) ValidateUsername(username string) error {
	if username == "" {
		return &apperrors.ValidationError{
			Field:   "username",
			Value:   username,
			Message: "username cannot be empty",
		}
	}

	if len(username) > 39 {
		return &apperrors.ValidationError{
			Field:   "username",
			Value:   username,
			Message: "username cannot exceed 39 characters",
		}
	}

	if !v.usernamePattern.MatchString(username) {
		return &apperrors.ValidationError{
			Field:   "username",
			Value:   username,
			Message: "username contains invalid characters",
		}
	}

	if strings.HasPrefix(username, "-") || strings.HasSuffix(username, "-") {
		return &apperrors.ValidationError{
			Field:   "username",
			Value:   username,
			Message: "username cannot start or end with hyphen",
		}
	}

	return nil
}

// ValidateGitHubToken validates GitHub token format.
// Accepts classic PATs (ghp_), fine-grained PATs (github_pat_), and other
// GitHub token types (gho_, ghu_, ghs_, ghr_) with a minimum length check.
func (v *GitHubValidator) ValidateGitHubToken(token string) error {
	if token == "" {
		return &apperrors.ValidationError{
			Field:   "github_token",
			Value:   "[REDACTED]",
			Message: "GitHub token cannot be empty",
		}
	}

	validPrefixes := []string{"ghp_", "gho_", "ghu_", "ghs_", "ghr_", "github_pat_"}
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(token, prefix) {
			if len(token) < 20 {
				return &apperrors.ValidationError{
					Field:   "github_token",
					Value:   "[REDACTED]",
					Message: "GitHub token appears too short to be valid",
				}
			}
			return nil
		}
	}

	return &apperrors.ValidationError{
		Field:   "github_token",
		Value:   "[REDACTED]",
		Message: "GitHub token format is not recognized (expected ghp_, gho_, ghu_, ghs_, ghr_, or github_pat_ prefix)",
	}
}

// ValidateEmail validates email format
func (v *GitHubValidator) ValidateEmail(email string) error {
	if email == "" {
		return &apperrors.ValidationError{
			Field:   "email",
			Value:   email,
			Message: "email cannot be empty",
		}
	}

	emailPattern := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	if !emailPattern.MatchString(email) {
		return &apperrors.ValidationError{
			Field:   "email",
			Value:   email,
			Message: "invalid email format",
		}
	}

	return nil
}

// ValidateURL validates URL format
func (v *GitHubValidator) ValidateURL(rawURL string) error {
	if rawURL == "" {
		return &apperrors.ValidationError{
			Field:   "url",
			Value:   rawURL,
			Message: "URL cannot be empty",
		}
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return &apperrors.ValidationError{
			Field:   "url",
			Value:   rawURL,
			Message: fmt.Sprintf("invalid URL format: %v", err),
		}
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return &apperrors.ValidationError{
			Field:   "url",
			Value:   rawURL,
			Message: "URL must use http or https scheme",
		}
	}

	return nil
}

// ValidateRegexPattern validates regex patterns for safety
func (v *GitHubValidator) ValidateRegexPattern(pattern string) error {
	if pattern == "" {
		return &apperrors.ValidationError{
			Field:   "regex_pattern",
			Value:   pattern,
			Message: "regex pattern cannot be empty",
		}
	}

	// Check for potentially dangerous patterns
	dangerousPatterns := []string{
		".*",      // Too broad
		".+",      // Too broad
		".*.*",    // Redundant
		"(.*)\\1", // Potential ReDoS
		"(a+)+",   // Potential ReDoS
		"(a|a)*",  // Potential ReDoS
	}

	for _, dangerous := range dangerousPatterns {
		if pattern == dangerous {
			return &apperrors.ValidationError{
				Field:   "regex_pattern",
				Value:   pattern,
				Message: fmt.Sprintf("pattern '%s' is potentially dangerous", dangerous),
			}
		}
	}

	// Validate regex syntax
	if _, err := regexp.Compile(pattern); err != nil {
		return &apperrors.ValidationError{
			Field:   "regex_pattern",
			Value:   pattern,
			Message: fmt.Sprintf("invalid regex syntax: %v", err),
		}
	}

	return nil
}

// IsValidOrgName provides a simple bool check for organization name validity
func IsValidOrgName(org string) bool {
	if org == "" || len(org) > 39 {
		return false
	}
	pattern := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9_-]*[a-zA-Z0-9])?$`)
	return pattern.MatchString(org)
}
