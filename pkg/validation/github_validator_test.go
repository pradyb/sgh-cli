// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package validation

import (
	"errors"
	"strings"
	"testing"

	"github.com/pradyb/sgh-cli/pkg/apperrors"
)

// assertValidationError checks that err is a *apperrors.ValidationError for the
// given field. Every validator in this package is documented as returning that
// type, and callers switch on it, so the concrete type is part of the contract.
func assertValidationError(t *testing.T, err error, wantField string) {
	t.Helper()

	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	var vErr *apperrors.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected *apperrors.ValidationError, got %T: %v", err, err)
	}
	if vErr.Field != wantField {
		t.Errorf("Field = %q, want %q", vErr.Field, wantField)
	}
	if vErr.Message == "" {
		t.Error("Message is empty; callers surface this to users")
	}
}

func TestValidateOrganizationName(t *testing.T) {
	v := NewGitHubValidator()

	tests := []struct {
		name    string
		org     string
		wantErr bool
	}{
		{"simple", "my-org", false},
		{"alphanumeric", "org123", false},
		{"with dots", "my.org", false},
		{"with underscore", "my_org", false},
		{"single char", "a", false},
		{"max length 39", strings.Repeat("a", 39), false},

		{"empty", "", true},
		{"too long at 40", strings.Repeat("a", 40), true},
		{"leading hyphen", "-myorg", true},
		{"trailing hyphen", "myorg-", true},
		{"space", "my org", true},
		{"slash", "my/org", true},
		{"at sign", "my@org", true},
		{"unicode", "my-örg", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateOrganizationName(tt.org)
			if tt.wantErr {
				assertValidationError(t, err, "organization_name")
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.org, err)
			}
		})
	}
}

func TestValidateRepositoryName(t *testing.T) {
	v := NewGitHubValidator()

	tests := []struct {
		name    string
		repo    string
		wantErr bool
	}{
		{"simple", "my-repo", false},
		{"dots", "my.repo.name", false},
		{"underscores", "my_repo", false},
		{"max length 100", strings.Repeat("a", 100), false},
		// A repository legitimately may start or end with a hyphen, unlike an
		// organization — the validator must not over-reject here.
		{"leading hyphen allowed", "-repo", false},

		{"empty", "", true},
		{"too long at 101", strings.Repeat("a", 101), true},
		{"space", "my repo", true},
		{"slash", "my/repo", true},

		{"reserved dot", ".", true},
		{"reserved dotdot", "..", true},
		{"reserved .git", ".git", true},
		{"reserved .github", ".github", true},
		{"reserved is case-insensitive", ".GIT", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateRepositoryName(tt.repo)
			if tt.wantErr {
				assertValidationError(t, err, "repository_name")
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.repo, err)
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	v := NewGitHubValidator()

	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{"simple", "john-doe", false},
		{"alphanumeric", "user123", false},
		{"max length 39", strings.Repeat("a", 39), false},

		{"empty", "", true},
		{"too long at 40", strings.Repeat("a", 40), true},
		{"leading hyphen", "-john", true},
		{"trailing hyphen", "john-", true},
		// GitHub usernames permit neither dots nor underscores.
		{"dot rejected", "john.doe", true},
		{"underscore rejected", "john_doe", true},
		{"space", "john doe", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateUsername(tt.username)
			if tt.wantErr {
				assertValidationError(t, err, "username")
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.username, err)
			}
		})
	}
}

func TestValidateGitHubToken(t *testing.T) {
	v := NewGitHubValidator()

	// 36 characters, matching the real shape of a classic PAT suffix.
	suffix := strings.Repeat("a", 36)

	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"classic pat", "ghp_" + suffix, false},
		{"oauth", "gho_" + suffix, false},
		{"user-to-server", "ghu_" + suffix, false},
		{"server-to-server", "ghs_" + suffix, false},
		{"refresh", "ghr_" + suffix, false},
		{"fine-grained", "github_pat_" + suffix, false},
		{"exactly 20 chars", "ghp_" + strings.Repeat("a", 16), false},

		{"empty", "", true},
		{"no recognized prefix", strings.Repeat("a", 40), true},
		{"19 chars is too short", "ghp_" + strings.Repeat("a", 15), true},
		{"prefix alone", "ghp_", true},
		{"wrong prefix", "gh_" + suffix, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateGitHubToken(tt.token)
			if tt.wantErr {
				assertValidationError(t, err, "github_token")
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// A validation error carries the offending value so it can be shown to the
// user. For tokens that would print the secret, so it must be redacted.
func TestValidateGitHubTokenNeverEchoesTheToken(t *testing.T) {
	v := NewGitHubValidator()
	const secret = "ghp_thisisasecretvaluethatmustnotleak12"

	err := v.ValidateGitHubToken(secret)
	if err == nil {
		t.Skip("token considered valid; nothing to redact")
	}

	var vErr *apperrors.ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("expected *apperrors.ValidationError, got %T", err)
	}
	if strings.Contains(vErr.Error(), secret) {
		t.Errorf("error text leaks the token: %s", vErr.Error())
	}
	if vErr.Value != "[REDACTED]" {
		t.Errorf("Value = %q, want %q", vErr.Value, "[REDACTED]")
	}
}

func TestValidateEmail(t *testing.T) {
	v := NewGitHubValidator()

	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"simple", "user@example.com", false},
		{"plus tag", "user+tag@example.com", false},
		{"dots", "first.last@example.com", false},
		{"subdomain", "user@mail.example.com", false},
		{"digits in domain", "user@example123.com", false},

		{"empty", "", true},
		{"no at sign", "userexample.com", true},
		{"no domain", "user@", true},
		{"no local part", "@example.com", true},
		{"no tld", "user@example", true},
		{"single char tld", "user@example.c", true},
		{"space", "user name@example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateEmail(tt.email)
			if tt.wantErr {
				assertValidationError(t, err, "email")
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.email, err)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	v := NewGitHubValidator()

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"https", "https://api.github.com", false},
		{"http", "http://localhost:8080", false},
		{"with path and query", "https://api.github.com/repos?page=2", false},

		{"empty", "", true},
		{"ftp scheme", "ftp://example.com", true},
		{"file scheme", "file:///etc/passwd", true},
		{"no scheme", "api.github.com", true},
		{"control character", "https://example.com/\x7f", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateURL(tt.url)
			if tt.wantErr {
				assertValidationError(t, err, "url")
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.url, err)
			}
		})
	}
}

func TestValidateRegexPattern(t *testing.T) {
	v := NewGitHubValidator()

	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{"anchored prefix", "^api-", false},
		{"anchored suffix", ".*-legacy$", false},
		{"alternation", "^(api|service)-", false},
		{"character class", "[a-z]+", false},

		{"empty", "", true},
		{"bare dot star", ".*", true},
		{"bare dot plus", ".+", true},
		{"redundant", ".*.*", true},
		{"redos nested quantifier", "(a+)+", true},
		{"redos alternation", "(a|a)*", true},

		{"unclosed group", "^(api-", true},
		{"unclosed class", "[a-z", true},
		{"dangling quantifier", "*abc", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateRegexPattern(tt.pattern)
			if tt.wantErr {
				assertValidationError(t, err, "regex_pattern")
				return
			}
			if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.pattern, err)
			}
		})
	}
}

func TestIsValidOrgName(t *testing.T) {
	tests := []struct {
		name string
		org  string
		want bool
	}{
		{"simple", "my-org", true},
		{"alphanumeric", "org123", true},
		{"underscore inside", "my_org", true},
		{"single char", "a", true},
		{"max length 39", strings.Repeat("a", 39), true},

		{"empty", "", false},
		{"too long at 40", strings.Repeat("a", 40), false},
		{"leading hyphen", "-myorg", false},
		{"trailing hyphen", "myorg-", false},
		{"leading underscore", "_myorg", false},
		{"space", "my org", false},
		// IsValidOrgName is stricter than ValidateOrganizationName, which
		// accepts dots. Callers must not treat the two as interchangeable.
		{"dot rejected here", "my.org", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidOrgName(tt.org); got != tt.want {
				t.Errorf("IsValidOrgName(%q) = %v, want %v", tt.org, got, tt.want)
			}
		})
	}
}
