package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lithammer/fuzzysearch/fuzzy"
	logger "github.com/prady-lab/sgh-cli/utils"
)

const DefaultFilename = "sgh.json"

type Config struct {
	Verbose       bool           `json:"verbose"`
	Organizations []Organization `json:"organizations"`
	orgData       map[string]Organization
}

type Organization struct {
	Name                 string                `json:"name"`
	Repositories         []string              `json:"repositories"`
	RepoPatterns         IncludeExcludePattern `json:"repo_patterns"`
	PullRequestAssignees []string              `json:"pull_request_assignees"`
	Tagger               Tagger                `json:"tagger"`
	ProtectedBranch      ProtectedBranch       `json:"protected_branch"`
}

type IncludeExcludePattern struct {
	Exclude []string `json:"exclude"`
	Include []string `json:"include"`
}

type Tagger struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ProtectedBranch struct {
	IgnoreBuildStatusCheckRepos []string `json:"ignore_build_status_check_repos"`
	BypassPullRequestUsers      []string `json:"bypass_pull_request_users"`
	AllowedRestrictionsUsers    []string `json:"allowed_restrictions_users"`
	ApprovingReviewCount        int      `json:"approving_review_count"`
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
	return config.orgData[strings.ToLower(orgName)].Repositories
}

func (config *Config) PullRequestAssignees(orgName string) []string {
	return config.orgData[strings.ToLower(orgName)].PullRequestAssignees
}

func (config *Config) TaggerName(orgName string) string {
	return config.orgData[strings.ToLower(orgName)].Tagger.Name
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
			actualRepoNames = append(actualRepoNames, fuzzyNames[0])
			if len(fuzzyNames) > 1 {
				logger.Glog.Warn().Str("matched names", strings.Join(fuzzyNames, ",")).Str("selected", fuzzyNames[0]).Msgf("Multiple Repos found for the search string %s", repoName)
			}
		} else {
			actualRepoNames = append(actualRepoNames, repoName)
		}
	}
	return actualRepoNames
}

func (config *Config) CanSelectRepositoryForProcessing(orgName, repoName string) bool {
	for _, org := range config.Organizations {
		if strings.EqualFold(org.Name, orgName) {
			if org.RepoPatterns.Include != nil {
				if matchPatterns(org.RepoPatterns.Include, repoName) {
					return true
				}
			}
			if org.RepoPatterns.Exclude != nil {
				return !matchPatterns(org.RepoPatterns.Exclude, repoName)
			}
		}
	}
	return false
}

func (config *Config) SetVerbose(verbose bool) {
	config.Verbose = verbose
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
	contents, err := os.ReadFile(configFile())
	if err != nil {
		if os.IsNotExist(err) {
			logger.Glog.Trace().
				Msgf("Config file not found: %s, fallback to default values", configFile())
			return nil
		}
		return err
	}

	err = json.Unmarshal(contents, config)
	if err != nil {
		return err
	}
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
	return filepath.Join(configDir(), DefaultFilename)
}

func configDir() string {
	d, _ := os.UserHomeDir()
	return d
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
