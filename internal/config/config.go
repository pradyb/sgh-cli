package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"

	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/utils"
)

const DefaultFilename = "sgh.json"

type Config struct {
	NoOfWorkers   int                     `json:"no_of_workers,omitempty"`
	Organizations []Organization          `json:"organizations"`
	orgData       map[string]Organization `json:"-"`
}

type Organization struct {
	Name                 string                `json:"name"`
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

type ProtectedBranch struct {
	IgnoreBuildStatusCheckRepos []string `json:"ignore_build_status_check_repos,omitempty"`
	BypassPullRequestUsers      []string `json:"bypass_pull_request_users,omitempty"`
	AllowedRestrictionsUsers    []string `json:"allowed_restrictions_users,omitempty"`
	ApprovingReviewCount        int      `json:"approving_review_count,omitempty"`
}

func Init() (*Config, error) {
	config := &Config{}
	if err := config.Load(); err != nil {
		return nil, err
	}
	config.orgData = make(map[string]Organization)
	for _, org := range config.Organizations {
		config.orgData[strings.ToLower(org.Name)] = org
	}
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
		logger.Glog.Warn().Str("org", orgName).Msg("Config not initialized")
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
	return config.orgData[strings.ToLower(orgName)].Tagger.Email
}

func (config *Config) ProtectedBranchDetail(orgName string) ProtectedBranch {
	return config.orgData[strings.ToLower(orgName)].ProtectedBranch
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

func (config *Config) AddOrganization(orgName string) bool {
	if !config.IsOrganizationPresent(orgName) {
		config.Organizations = append(config.Organizations, Organization{Name: orgName})
		return true
	}
	return false
}

func (config *Config) AddRepository(orgName, repoName string) bool {
	for i, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			if !config.IsRepositoryPresent(orgName, repoName) {
				config.Organizations[i].Repositories = append(org.Repositories, repoName)
				return true
			} else {
				return false
			}
		}
	}
	config.Organizations = append(
		config.Organizations,
		Organization{Name: orgName, Repositories: []string{repoName}},
	)
	return true
}

func (config *Config) AddPullRequestAssignee(orgName, pullRequestAssignee string) bool {
	for i, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			if !config.IsPullRequestAssigneePresent(orgName, pullRequestAssignee) {
				config.Organizations[i].PullRequestAssignees = append(org.PullRequestAssignees, pullRequestAssignee)
				return true
			}
			return false
		}
	}
	return false
}

func (config *Config) AddRepositoryPattern(orgName string, include bool, exclude bool, pattern string) bool {
	for i, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			if include && !config.IsRepositoryPatternPresent(orgName, pattern, true) {
				config.Organizations[i].RepoPatterns.Include = append(org.RepoPatterns.Include, pattern)
				return true
			} else if exclude && !config.IsRepositoryPatternPresent(orgName, pattern, false) {
				config.Organizations[i].RepoPatterns.Exclude = append(org.RepoPatterns.Exclude, pattern)
				return true
			}
			return false
		}
	}
	return false
}

func (config *Config) ActualRepositoryNamesUsingFzf(orgName string, repos []string) []string {
	actualRepoNames := make([]string, 0)
	configRepoNames := config.RepositoriesNames(orgName)
	if len(configRepoNames) == 0 {
		return repos
	}
	for _, repoName := range repos {
		fuzzyNames := fuzzy.Find(repoName, configRepoNames)
		if len(fuzzyNames) > 0 {
			if len(fuzzyNames) == 1 {
				actualRepoNames = append(actualRepoNames, fuzzyNames[0])
			} else if len(fuzzyNames) > 1 {
				if slices.Contains(fuzzyNames, repoName) {
					actualRepoNames = append(actualRepoNames, repoName)
				} else {
					actualRepoNames = append(actualRepoNames, fuzzyNames[0])
					logger.Glog.Warn().Str("matched names", strings.Join(fuzzyNames, ",")).Str("selected", fuzzyNames[0]).Msgf("Multiple Repos found for the search string %s", repoName)
				}
			}
		} else {
			actualRepoNames = append(actualRepoNames, repoName)
		}
	}
	return actualRepoNames
}

func (config *Config) CanSelectRepositoryForProcessing(orgName, repoName string) bool {
	repoPatterns := config.orgData[strings.ToLower(orgName)].RepoPatterns
	if repoPatterns.Include == nil && repoPatterns.Exclude == nil {
		return true
	}
	if repoPatterns.Include != nil {
		if matchPatterns(repoPatterns.Include, repoName) {
			if repoPatterns.Exclude != nil {
				return !matchPatterns(repoPatterns.Exclude, repoName)
			}
			return true
		}
	}
	if repoPatterns.Exclude != nil {
		return !matchPatterns(repoPatterns.Exclude, repoName)
	}
	return false
}

func (config *Config) SetTaggerName(orgName, taggerName string) {
	for i, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			config.Organizations[i].Tagger.Name = taggerName
		}
	}
}

func (config *Config) SetTaggerEmail(orgName, taggerEmail string) {
	for i, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			config.Organizations[i].Tagger.Email = taggerEmail
		}
	}
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

	err = os.WriteFile(configFile(), contents, 0o644)
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

func matchPatterns(patterns []string, repoName string) bool {
	for _, configRepo := range patterns {
		r, err := regexp.Compile(configRepo)
		if err != nil {
			logger.Glog.Error().Err(err).Msgf("Error in compiling the regex pattern: %s", configRepo)
			return false
		}
		if r.MatchString(repoName) {
			return true
		}
	}
	return false
}

// validate checks the configuration for common issues
func (config *Config) validate() error {
	// Validate worker count
	if config.NoOfWorkers < 0 {
		return fmt.Errorf("no_of_workers must be non-negative, got %d", config.NoOfWorkers)
	}
	if config.NoOfWorkers > 100 {
		logger.Glog.Warn().
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
		if !isValidOrgName(org.Name) {
			return fmt.Errorf("invalid organization name '%s': must contain only alphanumeric characters, hyphens, and underscores", org.Name)
		}

		// Validate protected branch settings
		if org.ProtectedBranch.ApprovingReviewCount < 0 {
			return fmt.Errorf("organization '%s' has negative approving_review_count: %d", org.Name, org.ProtectedBranch.ApprovingReviewCount)
		}
		if org.ProtectedBranch.ApprovingReviewCount > 10 {
			logger.Glog.Warn().
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

// validateRepoPatterns validates repository patterns for security
func validateRepoPatterns(patterns IncludeExcludePattern) error {
	allPatterns := append(patterns.Include, patterns.Exclude...)

	for _, pattern := range allPatterns {
		// Check for potentially dangerous patterns
		if strings.Contains(pattern, "..") {
			return fmt.Errorf("pattern '%s' contains '..' which could be dangerous", pattern)
		}

		// Check for overly broad patterns that might match too many repos
		if pattern == "*" || pattern == ".*" {
			return fmt.Errorf("pattern '%s' is too broad and could match all repositories", pattern)
		}

		// Validate regex pattern
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid regex pattern '%s': %w", pattern, err)
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
	pattern := `^[a-zA-Z0-9]([a-zA-Z0-9-_]*[a-zA-Z0-9])?$`
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

// isValidOrgName validates GitHub organization name format
func isValidOrgName(org string) bool {
	if org == "" || len(org) > 39 {
		return false
	}
	pattern := `^[a-zA-Z0-9]([a-zA-Z0-9_-]*[a-zA-Z0-9])?$`
	matched, _ := regexp.MatchString(pattern, org)
	return matched
}

// isValidEmail validates email format
func isValidEmail(email string) bool {
	pattern := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	matched, _ := regexp.MatchString(pattern, email)
	return matched
}
