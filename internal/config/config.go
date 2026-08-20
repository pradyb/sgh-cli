// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/lithammer/fuzzysearch/fuzzy"

	"github.com/pradyb/sgh-cli/pkg/logger"
	"github.com/pradyb/sgh-cli/pkg/ui"
	"github.com/pradyb/sgh-cli/pkg/validation"
	"github.com/pradyb/sgh-cli/utils"
)

const DefaultFilename = "sgh.json"

type Config struct {
	NoOfWorkers      int                       `json:"no_of_workers,omitempty"`
	Organizations    []Organization            `json:"organizations"`
	orgData          map[string]Organization   `json:"-"`
	compiledPatterns map[string]*regexp.Regexp `json:"-"`
	ownerTypeMu      sync.Mutex                `json:"-"`
}

type Organization struct {
	Name                 string                `json:"name"`
	Token                string                `json:"token,omitempty"`
	OwnerType            string                `json:"owner_type,omitempty"`
	Repositories         []string              `json:"repositories,omitempty"`
	RepoPatterns         IncludeExcludePattern `json:"repo_patterns,omitempty"`
	PullRequestAssignees []string              `json:"pull_request_assignees,omitempty"`
	Tagger               Tagger                `json:"tagger,omitempty"`
	ProtectedBranch      ProtectedBranch       `json:"protected_branch,omitempty"`
}

type IncludeExcludePattern struct {
	Exclude []string `json:"exclude,omitempty"`
	Include []string `json:"include,omitempty"`
}

type Tagger struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

type StatusCheck struct {
	Context string `json:"context"`
	AppID   int    `json:"app_id,omitempty"`
}

type ProtectedBranch struct {
	IgnoreBuildStatusCheckRepos []string      `json:"ignore_build_status_check_repos,omitempty"`
	BypassPullRequestUsers      []string      `json:"bypass_pull_request_users,omitempty"`
	AllowedRestrictionsUsers    []string      `json:"allowed_restrictions_users,omitempty"`
	ApprovingReviewCount        int           `json:"approving_review_count,omitempty"`
	StatusChecks                []StatusCheck `json:"status_checks,omitempty"`
}

func Init() (*Config, error) {
	config := &Config{}
	if err := config.Load(); err != nil {
		return nil, err
	}
	config.rebuildOrgData()
	return config, nil
}

func (config *Config) OrganizationNames() []string {
	var orgNames []string
	for _, org := range config.Organizations {
		orgNames = append(orgNames, org.Name)
	}
	return orgNames
}

func (config *Config) RepositoriesNames(orgName string) []string {
	if config == nil || config.orgData == nil {
		logger.Flog.Warn().Str("org", orgName).Msg("Config not initialized")
		return []string{}
	}
	org, exists := config.orgData[strings.ToLower(orgName)]
	if !exists {
		logger.Glog.Debug().Str("org", orgName).Msg("Organization not found in config")
		return []string{}
	}
	return org.Repositories
}

func (config *Config) IncludePatterns(orgName string) []string {
	if config == nil || config.orgData == nil {
		return []string{}
	}
	org, exists := config.orgData[strings.ToLower(orgName)]
	if !exists {
		return []string{}
	}
	return org.RepoPatterns.Include
}

func (config *Config) ExcludePatterns(orgName string) []string {
	if config == nil || config.orgData == nil {
		return []string{}
	}
	org, exists := config.orgData[strings.ToLower(orgName)]
	if !exists {
		return []string{}
	}
	return org.RepoPatterns.Exclude
}

func (config *Config) PullRequestAssignees(orgName string) []string {
	if config == nil || config.orgData == nil {
		return []string{}
	}
	org, exists := config.orgData[strings.ToLower(orgName)]
	if !exists {
		return []string{}
	}
	return org.PullRequestAssignees
}

func (config *Config) TaggerName(orgName string) string {
	if config == nil || config.orgData == nil {
		return ""
	}
	org, exists := config.orgData[strings.ToLower(orgName)]
	if !exists {
		return ""
	}
	return org.Tagger.Name
}

func (config *Config) TaggerEmail(orgName string) string {
	if config == nil || config.orgData == nil {
		return ""
	}
	org, exists := config.orgData[strings.ToLower(orgName)]
	if !exists {
		return ""
	}
	return org.Tagger.Email
}

func (config *Config) ProtectedBranchDetail(orgName string) ProtectedBranch {
	if config == nil || config.orgData == nil {
		return ProtectedBranch{}
	}
	org, exists := config.orgData[strings.ToLower(orgName)]
	if !exists {
		return ProtectedBranch{}
	}
	return org.ProtectedBranch
}

// OwnerTypeFor returns the cached owner type ("Organization" or "User") for the given name,
// or "" if it has not been detected yet.
func (config *Config) OwnerTypeFor(orgName string) string {
	if config == nil {
		return ""
	}
	config.ownerTypeMu.Lock()
	defer config.ownerTypeMu.Unlock()
	if config.orgData == nil {
		return ""
	}
	org, exists := config.orgData[strings.ToLower(orgName)]
	if !exists {
		return ""
	}
	return org.OwnerType
}

// SetOwnerType caches the detected owner type for the given name.
func (config *Config) SetOwnerType(orgName, ownerType string) {
	config.ownerTypeMu.Lock()
	defer config.ownerTypeMu.Unlock()
	for i, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			config.Organizations[i].OwnerType = ownerType
			config.rebuildOrgData()
			return
		}
	}
	config.Organizations = append(config.Organizations, Organization{Name: orgName, OwnerType: ownerType})
	config.rebuildOrgData()
}

func (config *Config) TokenForOwner(orgName string) string {
	if config == nil || config.orgData == nil {
		return ""
	}
	org, exists := config.orgData[strings.ToLower(orgName)]
	if !exists {
		return ""
	}
	return org.Token
}

func (config *Config) IsOrganizationPresent(orgName string) bool {
	_, ok := config.orgData[strings.ToLower(orgName)]
	return ok
}

func (config *Config) IsRepositoryPresent(orgName, repoName string) bool {
	for _, configRepoName := range config.RepositoriesNames(orgName) {
		if strings.EqualFold(configRepoName, repoName) {
			return true
		}
	}
	return false
}

func (config *Config) IsPullRequestAssigneePresent(orgName, assignee string) bool {
	for _, a := range config.PullRequestAssignees(orgName) {
		if strings.EqualFold(a, assignee) {
			return true
		}
	}
	return false
}

func (config *Config) IsRepoPresentInIgnoreForStatusCheck(orgName, repoName string) bool {
	for _, a := range config.ProtectedBranchDetail(orgName).IgnoreBuildStatusCheckRepos {
		if strings.EqualFold(a, repoName) {
			return true
		}
	}
	return false
}

func (config *Config) IsRepositoryPatternPresent(orgName, pattern string, inIncludePattern bool) bool {
	for _, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			if inIncludePattern && isPatternPresent(org.RepoPatterns.Include, pattern) {
				return true
			} else if !inIncludePattern && isPatternPresent(org.RepoPatterns.Exclude, pattern) {
				return true
			}
		}
	}
	return false
}

func isPatternPresent(patterns []string, pattern string) bool {
	for _, p := range patterns {
		if strings.EqualFold(p, pattern) {
			return true
		}
	}
	return false
}

func (config *Config) rebuildOrgData() {
	config.orgData = make(map[string]Organization, len(config.Organizations))
	config.compiledPatterns = make(map[string]*regexp.Regexp)
	for _, org := range config.Organizations {
		config.orgData[strings.ToLower(org.Name)] = org
		for _, p := range org.RepoPatterns.Include {
			if _, exists := config.compiledPatterns[p]; !exists {
				if r, err := regexp.Compile(p); err == nil {
					config.compiledPatterns[p] = r
				} else {
					logger.Flog.Error().Err(err).Str("pattern", p).Msg("Failed to compile regex pattern")
				}
			}
		}
		for _, p := range org.RepoPatterns.Exclude {
			if _, exists := config.compiledPatterns[p]; !exists {
				if r, err := regexp.Compile(p); err == nil {
					config.compiledPatterns[p] = r
				} else {
					logger.Flog.Error().Err(err).Str("pattern", p).Msg("Failed to compile regex pattern")
				}
			}
		}
	}
}

func (config *Config) AddOrganization(orgName string) bool {
	if !config.IsOrganizationPresent(orgName) {
		config.Organizations = append(config.Organizations, Organization{Name: orgName})
		config.rebuildOrgData()
		return true
	}
	return false
}

func (config *Config) AddRepository(orgName, repoName string) bool {
	for i, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			if !config.IsRepositoryPresent(orgName, repoName) {
				config.Organizations[i].Repositories = append(config.Organizations[i].Repositories, repoName)
				config.rebuildOrgData()
				return true
			}
			return false
		}
	}
	config.Organizations = append(
		config.Organizations,
		Organization{Name: orgName, Repositories: []string{repoName}},
	)
	config.rebuildOrgData()
	return true
}

func (config *Config) AddPullRequestAssignee(orgName, pullRequestAssignee string) bool {
	for i, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			if !config.IsPullRequestAssigneePresent(orgName, pullRequestAssignee) {
				config.Organizations[i].PullRequestAssignees = append(config.Organizations[i].PullRequestAssignees, pullRequestAssignee)
				config.rebuildOrgData()
				return true
			}
			return false
		}
	}
	return false
}

func (config *Config) AddRepositoryPattern(orgName string, include bool, exclude bool, pattern string) bool {
	// Auto-create org if it doesn't exist, consistent with AddRepository behaviour.
	if !config.IsOrganizationPresent(orgName) {
		config.Organizations = append(config.Organizations, Organization{Name: orgName})
		config.rebuildOrgData()
		logger.Flog.Info().Str("org", orgName).Msg("Auto-created organization for pattern")
	}

	for i, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			if include && !config.IsRepositoryPatternPresent(orgName, pattern, true) {
				config.Organizations[i].RepoPatterns.Include = append(config.Organizations[i].RepoPatterns.Include, pattern)
				config.rebuildOrgData()
				return true
			} else if exclude && !config.IsRepositoryPatternPresent(orgName, pattern, false) {
				config.Organizations[i].RepoPatterns.Exclude = append(config.Organizations[i].RepoPatterns.Exclude, pattern)
				config.rebuildOrgData()
				return true
			}
			return false
		}
	}
	return false
}

// RemoveOrganization removes an organization and all its associated data from the
// config. Returns true if the org was found and removed, false if it was not present.
func (config *Config) RemoveOrganization(orgName string) bool {
	for i, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			config.Organizations = append(config.Organizations[:i], config.Organizations[i+1:]...)
			config.rebuildOrgData()
			return true
		}
	}
	return false
}

// RemoveRepository removes a repository from an organization's list.
// Returns true if removed, false if the org or repo was not found.
func (config *Config) RemoveRepository(orgName, repoName string) bool {
	for i, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			for j, r := range org.Repositories {
				if strings.EqualFold(r, repoName) {
					config.Organizations[i].Repositories = append(
						config.Organizations[i].Repositories[:j],
						config.Organizations[i].Repositories[j+1:]...,
					)
					config.rebuildOrgData()
					return true
				}
			}
			return false
		}
	}
	return false
}

// RemovePullRequestAssignee removes an assignee from an org's PR assignee list.
// Returns true if removed, false if not found.
func (config *Config) RemovePullRequestAssignee(orgName, assignee string) bool {
	for i, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			for j, a := range org.PullRequestAssignees {
				if strings.EqualFold(a, assignee) {
					config.Organizations[i].PullRequestAssignees = append(
						config.Organizations[i].PullRequestAssignees[:j],
						config.Organizations[i].PullRequestAssignees[j+1:]...,
					)
					config.rebuildOrgData()
					return true
				}
			}
			return false
		}
	}
	return false
}

// RemoveRepositoryPattern removes an include or exclude pattern from an org.
// Returns true if removed, false if not found.
func (config *Config) RemoveRepositoryPattern(orgName string, include bool, pattern string) bool {
	for i, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			if include {
				for j, p := range org.RepoPatterns.Include {
					if p == pattern {
						config.Organizations[i].RepoPatterns.Include = append(
							config.Organizations[i].RepoPatterns.Include[:j],
							config.Organizations[i].RepoPatterns.Include[j+1:]...,
						)
						config.rebuildOrgData()
						return true
					}
				}
			} else {
				for j, p := range org.RepoPatterns.Exclude {
					if p == pattern {
						config.Organizations[i].RepoPatterns.Exclude = append(
							config.Organizations[i].RepoPatterns.Exclude[:j],
							config.Organizations[i].RepoPatterns.Exclude[j+1:]...,
						)
						config.rebuildOrgData()
						return true
					}
				}
			}
			return false
		}
	}
	return false
}

func (config *Config) ActualRepositoryNamesUsingFzf(orgName string, repos []string) []string {
	actualRepoNames := make([]string, 0)
	seen := make(map[string]bool)
	configRepoNames := config.RepositoriesNames(orgName)
	if len(configRepoNames) == 0 {
		return repos
	}
	for _, repoName := range repos {
		resolved := resolveRepoName(repoName, configRepoNames)
		if !seen[resolved] {
			seen[resolved] = true
			actualRepoNames = append(actualRepoNames, resolved)
		}
	}
	return actualRepoNames
}

func resolveRepoName(repoName string, configRepoNames []string) string {
	// Prefer exact match (case-insensitive) — no ambiguity
	for _, cfg := range configRepoNames {
		if strings.EqualFold(cfg, repoName) {
			return cfg
		}
	}

	// Rank all fuzzy matches by Levenshtein distance (case-insensitive)
	ranked := fuzzy.RankFindFold(repoName, configRepoNames)
	if len(ranked) == 0 {
		ui.PrintNoFuzzyMatchWarning(repoName)
		return repoName
	}

	sort.Sort(ranked)
	best := ranked[0].Target

	if len(ranked) == 1 {
		return best
	}

	matchNames := make([]string, len(ranked))
	for i, r := range ranked {
		matchNames[i] = r.Target
	}
	logger.Flog.Warn().Str("search", repoName).Str("matched", strings.Join(matchNames, ",")).Str("selected", best).Msg("Multiple repos matched fuzzy search")
	ui.PrintFuzzyMatchWarning(repoName, matchNames, best)
	return best
}

// CanSelectRepositoryForProcessing decides whether a repo should be processed
// based on the configured include/exclude regex patterns for the org.
//
// Rules (evaluated in order):
//  1. No patterns configured → include everything.
//  2. Exclude patterns match → always exclude, regardless of include.
//  3. Include patterns configured → only include repos that match at least one.
//  4. Only exclude patterns configured → include everything not excluded.
func (config *Config) CanSelectRepositoryForProcessing(orgName, repoName string) bool {
	patterns := config.orgData[strings.ToLower(orgName)].RepoPatterns
	hasInclude := len(patterns.Include) > 0
	hasExclude := len(patterns.Exclude) > 0

	// Rule 1: no patterns at all → include everything
	if !hasInclude && !hasExclude {
		return true
	}

	// Rule 2: exclude always wins
	if hasExclude && config.matchPatterns(patterns.Exclude, repoName) {
		return false
	}

	// Rule 3: include filter is active — repo must match at least one include pattern
	if hasInclude {
		return config.matchPatterns(patterns.Include, repoName)
	}

	// Rule 4: only exclude patterns configured, repo didn't match any — include it
	return true
}

func (config *Config) SetToken(orgName, token string) {
	for i, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			config.Organizations[i].Token = token
			config.rebuildOrgData()
			return
		}
	}
	config.Organizations = append(config.Organizations, Organization{Name: orgName, Token: token})
	config.rebuildOrgData()
}

func (config *Config) SetTaggerName(orgName, taggerName string) {
	for i, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			config.Organizations[i].Tagger.Name = taggerName
		}
	}
	config.rebuildOrgData()
}

func (config *Config) SetTaggerEmail(orgName, taggerEmail string) {
	for i, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			config.Organizations[i].Tagger.Email = taggerEmail
		}
	}
	config.rebuildOrgData()
}

func (config *Config) Load() error {
	configPath := configFile()
	contents, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Glog.Debug().
				Str("configPath", configPath).
				Msg("Config file not found, using default values")
			return nil
		}
		return fmt.Errorf("failed to read config file '%s': %w", configPath, err)
	}

	if len(contents) == 0 {
		logger.Glog.Debug().
			Str("configPath", configPath).
			Msg("Config file is empty, using default values")
		return nil
	}

	err = json.Unmarshal(contents, config)
	if err != nil {
		return fmt.Errorf("failed to parse config file '%s': %w", configPath, err)
	}

	// Validate loaded configuration
	if err := config.validate(); err != nil {
		return fmt.Errorf("invalid configuration in '%s': %w", configPath, err)
	}

	logger.Glog.Debug().
		Str("configPath", configPath).
		Int("organizations", len(config.Organizations)).
		Msg("Successfully loaded configuration")

	return nil
}

func (config *Config) Save() error {
	contents, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		return err
	}

	err = os.WriteFile(configFile(), contents, 0o600)
	if err != nil {
		return err
	}

	return nil
}

func configFile() string {
	configDir, err := utils.ConfigDir()
	if err != nil {
		logger.Glog.Error().Err(err).Msg("Failed to get config directory")
		return DefaultFilename
	}
	return filepath.Join(configDir, DefaultFilename)
}

// ConfigFilePath returns the absolute path to the sgh config file.
func ConfigFilePath() string { return configFile() }

func (config *Config) matchPatterns(patterns []string, repoName string) bool {
	for _, pattern := range patterns {
		r, ok := config.compiledPatterns[pattern]
		if !ok {
			var err error
			r, err = regexp.Compile(pattern)
			if err != nil {
				logger.Flog.Error().Err(err).Str("pattern", pattern).Msg("Failed to compile regex pattern")
				continue
			}
			if config.compiledPatterns != nil {
				config.compiledPatterns[pattern] = r
			}
		}
		if r.MatchString(repoName) {
			return true
		}
	}
	return false
}

// validate checks the configuration for common issues
// Validate runs all configuration integrity checks and returns the first error found.
// It is safe to call at any time and does not mutate the config.
func (config *Config) Validate() error { return config.validate() }

func (config *Config) validate() error {
	// Validate worker count
	if config.NoOfWorkers < 0 {
		return fmt.Errorf("no_of_workers must be non-negative, got %d", config.NoOfWorkers)
	}
	if config.NoOfWorkers > 100 {
		logger.Flog.Warn().
			Int("workers", config.NoOfWorkers).
			Msg("Very high worker count may cause rate limiting issues")
	}

	// Validate organizations
	orgNames := make(map[string]bool)
	for i, org := range config.Organizations {
		if org.Name == "" {
			return fmt.Errorf("organization at index %d has empty name", i)
		}

		// Check for duplicate org names (case-insensitive)
		lowerName := strings.ToLower(org.Name)
		if orgNames[lowerName] {
			return fmt.Errorf("duplicate organization name: %s", org.Name)
		}
		orgNames[lowerName] = true

		// Enhanced organization name validation
		if !validation.IsValidOrgName(org.Name) {
			return fmt.Errorf("invalid organization name '%s': must contain only alphanumeric characters, hyphens, and underscores", org.Name)
		}

		// Validate protected branch settings
		if org.ProtectedBranch.ApprovingReviewCount < 0 {
			return fmt.Errorf("organization '%s' has negative approving_review_count: %d", org.Name, org.ProtectedBranch.ApprovingReviewCount)
		}
		if org.ProtectedBranch.ApprovingReviewCount > 10 {
			logger.Flog.Warn().
				Str("org", org.Name).
				Int("count", org.ProtectedBranch.ApprovingReviewCount).
				Msg("Very high approving review count may be impractical")
		}

		// Enhanced email validation
		if org.Tagger.Email != "" {
			if !isValidEmail(org.Tagger.Email) {
				return fmt.Errorf("organization '%s' has invalid tagger email: %s", org.Name, org.Tagger.Email)
			}
		}

		// Validate repository patterns for security
		if err := validateRepoPatterns(org.RepoPatterns); err != nil {
			return fmt.Errorf("organization '%s' has invalid repository patterns: %w", org.Name, err)
		}

		// Validate user lists for security
		if err := validateUserList(org.PullRequestAssignees, "pull_request_assignees"); err != nil {
			return fmt.Errorf("organization '%s' has invalid pull request assignees: %w", org.Name, err)
		}
		if err := validateUserList(org.ProtectedBranch.BypassPullRequestUsers, "bypass_pull_request_users"); err != nil {
			return fmt.Errorf("organization '%s' has invalid bypass users: %w", org.Name, err)
		}
		if err := validateUserList(org.ProtectedBranch.AllowedRestrictionsUsers, "allowed_restrictions_users"); err != nil {
			return fmt.Errorf("organization '%s' has invalid allowed restrictions users: %w", org.Name, err)
		}
	}

	return nil
}

// catchAllPatterns are regexes that match every possible string.
// We reject these to prevent accidental blanket inclusion/exclusion.
var catchAllPatterns = []string{
	`.*`, `.*?`, `.+`, `.+?`, `^.*$`, `^.+$`,
	`[\s\S]*`, `[\s\S]+`, `(?s:.*)`, `(?s:.+)`,
	`[^]*`, `*`,
}

// ValidatePattern validates a single include/exclude repository pattern.
// It is exported so the cmd layer can validate patterns before storing them,
// giving the user immediate, actionable feedback.
//
// Checks performed (in order):
//  1. Not empty or whitespace-only
//  2. Does not contain ".." (path traversal)
//  3. Not a known catch-all literal (e.g. ".*", ".+", "*")
//  4. Compiles as a valid Go regular expression
//  5. Does not behave as a catch-all at runtime (matches "" and any long string)
func ValidatePattern(pattern string) error {
	if strings.TrimSpace(pattern) == "" {
		return fmt.Errorf("pattern cannot be empty or whitespace-only")
	}

	if strings.Contains(pattern, "..") {
		return fmt.Errorf("pattern %q contains '..' which is not allowed", pattern)
	}

	for _, bad := range catchAllPatterns {
		if pattern == bad {
			return fmt.Errorf("pattern %q matches every repository — use a more specific pattern (e.g. %q)", pattern, "^my-service-")
		}
	}

	r, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("pattern %q is not a valid Go regular expression: %w", pattern, err)
	}

	// Runtime catch-all detection: matches empty string AND arbitrary content
	if r.MatchString("") && r.MatchString("arbitrary-repo-name-xyz-123") {
		return fmt.Errorf("pattern %q appears to match every repository — use a more specific pattern (e.g. %q)", pattern, "^my-service-")
	}

	return nil
}

// validateRepoPatterns validates all include and exclude patterns for an org.
func validateRepoPatterns(patterns IncludeExcludePattern) error {
	for _, p := range patterns.Include {
		if err := ValidatePattern(p); err != nil {
			return fmt.Errorf("include pattern: %w", err)
		}
	}
	for _, p := range patterns.Exclude {
		if err := ValidatePattern(p); err != nil {
			return fmt.Errorf("exclude pattern: %w", err)
		}
	}
	return nil
}

// validateUserList validates user lists for security
func validateUserList(users []string, fieldName string) error {
	for i, user := range users {
		if user == "" {
			return fmt.Errorf("empty user name at index %d in %s", i, fieldName)
		}

		// Check for valid GitHub username format
		if !isValidGitHubUsername(user) {
			return fmt.Errorf("invalid GitHub username '%s' at index %d in %s", user, i, fieldName)
		}
	}

	return nil
}

// isValidGitHubUsername validates GitHub username format
func isValidGitHubUsername(username string) bool {
	if username == "" || len(username) > 39 {
		return false
	}

	// GitHub usernames can contain alphanumeric characters and single hyphens
	// Cannot start or end with hyphen, and cannot have consecutive hyphens
	pattern := `^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$`
	matched, _ := regexp.MatchString(pattern, username)

	if !matched {
		return false
	}

	// Check for consecutive hyphens
	if strings.Contains(username, "--") {
		return false
	}

	return true
}

// isValidEmail validates email format
func isValidEmail(email string) bool {
	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	matched, _ := regexp.MatchString(pattern, email)
	return matched
}
