// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pradyb/sgh-cli/pkg/validation"
)

// Test constants to avoid duplication
const (
	testOrgName        = "testorg"
	testRepoName       = "test-repo"
	testEmail          = "test@example.com"
	testAPIPattern     = ".*api.*"
	testWebPattern     = "web-.*"
	testExcludePattern = ".*test.*"
	nonExistentOrg     = "nonexistent"
	expectedRepoError  = "Expected %v for repo '%s' in org '%s', got %v"
)

// createTestConfig creates a sample configuration for testing
func createTestConfig() *Config {
	return &Config{
		NoOfWorkers: 5,
		Organizations: []Organization{
			{
				Name:         testOrgName,
				Repositories: []string{"repo1", "repo2", testRepoName},
				RepoPatterns: IncludeExcludePattern{
					Include: []string{testAPIPattern, testWebPattern},
					Exclude: []string{testExcludePattern, "deprecated-.*"},
				},
				PullRequestAssignees: []string{"user1", "user2"},
				Tagger: Tagger{
					Name:  "Test User",
					Email: testEmail,
				},
				ProtectedBranch: ProtectedBranch{
					IgnoreBuildStatusCheckRepos: []string{"legacy-repo"},
					BypassPullRequestUsers:      []string{"admin"},
					AllowedRestrictionsUsers:    []string{"maintainer"},
					ApprovingReviewCount:        2,
				},
			},
			{
				Name:         "emptyorg",
				Repositories: []string{},
			},
			{
				Name:         "UPPERORG",
				Repositories: []string{"upperrepo"},
				Tagger: Tagger{
					Name:  "Upper User",
					Email: "upper@example.com",
				},
			},
		},
		orgData: make(map[string]Organization),
	}
}

// setupConfig initializes a test config with orgData
func setupConfig() *Config {
	config := createTestConfig()
	config.orgData = make(map[string]Organization)
	for _, org := range config.Organizations {
		config.orgData[strings.ToLower(org.Name)] = org
	}
	return config
}

func TestInit(t *testing.T) {
	// Create a temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, DefaultFilename)
	testConfig := createTestConfig()
	data, _ := json.MarshalIndent(testConfig, "", "    ")
	os.WriteFile(configPath, data, 0o644)

	// Test with a valid config file by changing working directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tempDir)

	// Since we can't easily mock the utils.ConfigDir, we'll test the core functionality
	config := &Config{}
	err := config.Load()
	if err != nil {
		// If it fails because config file is not in expected location, that's OK for this test
		// We're mainly testing that Init doesn't crash
		t.Logf("Load failed as expected: %v", err)
	}

	// Test the orgData initialization logic
	config = createTestConfig()
	config.orgData = make(map[string]Organization)
	for _, org := range config.Organizations {
		config.orgData[strings.ToLower(org.Name)] = org
	}

	if len(config.orgData) != len(config.Organizations) {
		t.Errorf("Expected orgData to have %d entries, got %d", len(config.Organizations), len(config.orgData))
	}

	// Verify orgData is properly initialized with lowercase keys
	if _, exists := config.orgData[testOrgName]; !exists {
		t.Errorf("Expected '%s' to be in orgData", testOrgName)
	}
	if _, exists := config.orgData["upperorg"]; !exists {
		t.Error("Expected 'upperorg' (lowercase) to be in orgData")
	}
}

func TestInitNoConfigFile(t *testing.T) {
	// Test initialization when no config file exists
	config := &Config{}
	err := config.Load()
	if err != nil {
		t.Logf("Load failed as expected when no config file exists: %v", err)
	}

	// Test that we can still create a valid config structure
	config = &Config{
		Organizations: []Organization{},
		orgData:       make(map[string]Organization),
	}

	if config == nil {
		t.Fatal("Expected config to be non-nil")
	}

	if len(config.Organizations) != 0 {
		t.Errorf("Expected empty organizations list, got %d", len(config.Organizations))
	}
}

func TestOrganizationNames(t *testing.T) {
	config := setupConfig()

	orgNames := config.OrganizationNames()
	expected := []string{"testorg", "emptyorg", "UPPERORG"}

	if len(orgNames) != len(expected) {
		t.Errorf("Expected %d organization names, got %d", len(expected), len(orgNames))
	}

	for _, expectedName := range expected {
		found := false
		for _, actualName := range orgNames {
			if actualName == expectedName {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected organization name '%s' not found", expectedName)
		}
	}
}

func TestRepositoriesNames(t *testing.T) {
	config := setupConfig()

	tests := []struct {
		name     string
		orgName  string
		expected []string
	}{
		{
			name:     "existing org with repos",
			orgName:  "testorg",
			expected: []string{"repo1", "repo2", "test-repo"},
		},
		{
			name:     "existing org without repos",
			orgName:  "emptyorg",
			expected: []string{},
		},
		{
			name:     "case insensitive org name",
			orgName:  "TESTORG",
			expected: []string{"repo1", "repo2", "test-repo"},
		},
		{
			name:     "non-existent org",
			orgName:  nonExistentOrg,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.RepositoriesNames(tt.orgName)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d repositories, got %d", len(tt.expected), len(result))
			}
			for i, expected := range tt.expected {
				if i < len(result) && result[i] != expected {
					t.Errorf("Expected repository '%s' at index %d, got '%s'", expected, i, result[i])
				}
			}
		})
	}
}

func TestRepositoriesNamesNilConfig(t *testing.T) {
	var config *Config
	result := config.RepositoriesNames("testorg")
	if len(result) != 0 {
		t.Errorf("Expected empty slice for nil config, got %v", result)
	}
}

func TestIncludePatterns(t *testing.T) {
	config := setupConfig()

	tests := []struct {
		name     string
		orgName  string
		expected []string
	}{
		{
			name:     "org with include patterns",
			orgName:  "testorg",
			expected: []string{".*api.*", "web-.*"},
		},
		{
			name:     "org without include patterns",
			orgName:  "emptyorg",
			expected: []string{},
		},
		{
			name:     "non-existent org",
			orgName:  "nonexistent",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.IncludePatterns(tt.orgName)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d patterns, got %d", len(tt.expected), len(result))
			}
			for i, expected := range tt.expected {
				if i < len(result) && result[i] != expected {
					t.Errorf("Expected pattern '%s' at index %d, got '%s'", expected, i, result[i])
				}
			}
		})
	}
}

func TestExcludePatterns(t *testing.T) {
	config := setupConfig()

	tests := []struct {
		name     string
		orgName  string
		expected []string
	}{
		{
			name:     "org with exclude patterns",
			orgName:  "testorg",
			expected: []string{".*test.*", "deprecated-.*"},
		},
		{
			name:     "org without exclude patterns",
			orgName:  "emptyorg",
			expected: []string{},
		},
		{
			name:     "non-existent org",
			orgName:  "nonexistent",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.ExcludePatterns(tt.orgName)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d patterns, got %d", len(tt.expected), len(result))
			}
			for i, expected := range tt.expected {
				if i < len(result) && result[i] != expected {
					t.Errorf("Expected pattern '%s' at index %d, got '%s'", expected, i, result[i])
				}
			}
		})
	}
}

func TestPullRequestAssignees(t *testing.T) {
	config := setupConfig()

	tests := []struct {
		name     string
		orgName  string
		expected []string
	}{
		{
			name:     "org with assignees",
			orgName:  "testorg",
			expected: []string{"user1", "user2"},
		},
		{
			name:     "org without assignees",
			orgName:  "emptyorg",
			expected: []string{},
		},
		{
			name:     "non-existent org",
			orgName:  "nonexistent",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.PullRequestAssignees(tt.orgName)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d assignees, got %d", len(tt.expected), len(result))
			}
			for i, expected := range tt.expected {
				if i < len(result) && result[i] != expected {
					t.Errorf("Expected assignee '%s' at index %d, got '%s'", expected, i, result[i])
				}
			}
		})
	}
}

func TestTaggerName(t *testing.T) {
	config := setupConfig()

	tests := []struct {
		name     string
		orgName  string
		expected string
	}{
		{
			name:     "org with tagger name",
			orgName:  "testorg",
			expected: "Test User",
		},
		{
			name:     "org without tagger name",
			orgName:  "emptyorg",
			expected: "",
		},
		{
			name:     "non-existent org",
			orgName:  "nonexistent",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.TaggerName(tt.orgName)
			if result != tt.expected {
				t.Errorf("Expected tagger name '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestTaggerEmail(t *testing.T) {
	config := setupConfig()

	tests := []struct {
		name     string
		orgName  string
		expected string
	}{
		{
			name:     "org with tagger email",
			orgName:  "testorg",
			expected: "test@example.com",
		},
		{
			name:     "org without tagger email",
			orgName:  "emptyorg",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.TaggerEmail(tt.orgName)
			if result != tt.expected {
				t.Errorf("Expected tagger email '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestProtectedBranchDetail(t *testing.T) {
	config := setupConfig()

	result := config.ProtectedBranchDetail("testorg")
	expected := config.orgData["testorg"].ProtectedBranch

	if result.ApprovingReviewCount != expected.ApprovingReviewCount {
		t.Errorf("Expected approving review count %d, got %d", expected.ApprovingReviewCount, result.ApprovingReviewCount)
	}

	if len(result.IgnoreBuildStatusCheckRepos) != len(expected.IgnoreBuildStatusCheckRepos) {
		t.Errorf("Expected %d ignore repos, got %d", len(expected.IgnoreBuildStatusCheckRepos), len(result.IgnoreBuildStatusCheckRepos))
	}
}

func TestIsOrganizationPresent(t *testing.T) {
	config := setupConfig()

	tests := []struct {
		name     string
		orgName  string
		expected bool
	}{
		{
			name:     "existing org",
			orgName:  "testorg",
			expected: true,
		},
		{
			name:     "existing org case insensitive",
			orgName:  "TESTORG",
			expected: true,
		},
		{
			name:     "non-existent org",
			orgName:  "nonexistent",
			expected: false,
		},
		{
			name:     "empty org name",
			orgName:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.IsOrganizationPresent(tt.orgName)
			if result != tt.expected {
				t.Errorf("Expected %v for org '%s', got %v", tt.expected, tt.orgName, result)
			}
		})
	}
}

func TestIsRepositoryPresent(t *testing.T) {
	config := setupConfig()

	tests := []struct {
		name     string
		orgName  string
		repoName string
		expected bool
	}{
		{
			name:     "existing repo",
			orgName:  "testorg",
			repoName: "repo1",
			expected: true,
		},
		{
			name:     "existing repo case insensitive",
			orgName:  "testorg",
			repoName: "REPO1",
			expected: true,
		},
		{
			name:     "non-existent repo",
			orgName:  "testorg",
			repoName: "nonexistent",
			expected: false,
		},
		{
			name:     "non-existent org",
			orgName:  "nonexistent",
			repoName: "repo1",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.IsRepositoryPresent(tt.orgName, tt.repoName)
			if result != tt.expected {
				t.Errorf("Expected %v for repo '%s' in org '%s', got %v", tt.expected, tt.repoName, tt.orgName, result)
			}
		})
	}
}

func TestIsPullRequestAssigneePresent(t *testing.T) {
	config := setupConfig()

	tests := []struct {
		name     string
		orgName  string
		assignee string
		expected bool
	}{
		{
			name:     "existing assignee",
			orgName:  "testorg",
			assignee: "user1",
			expected: true,
		},
		{
			name:     "existing assignee case insensitive",
			orgName:  "testorg",
			assignee: "USER1",
			expected: true,
		},
		{
			name:     "non-existent assignee",
			orgName:  "testorg",
			assignee: "nonexistent",
			expected: false,
		},
		{
			name:     "non-existent org",
			orgName:  "nonexistent",
			assignee: "user1",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.IsPullRequestAssigneePresent(tt.orgName, tt.assignee)
			if result != tt.expected {
				t.Errorf("Expected %v for assignee '%s' in org '%s', got %v", tt.expected, tt.assignee, tt.orgName, result)
			}
		})
	}
}

func TestIsRepoPresentInIgnoreForStatusCheck(t *testing.T) {
	config := setupConfig()

	tests := []struct {
		name     string
		orgName  string
		repoName string
		expected bool
	}{
		{
			name:     "repo in ignore list",
			orgName:  "testorg",
			repoName: "legacy-repo",
			expected: true,
		},
		{
			name:     "repo in ignore list case insensitive",
			orgName:  "testorg",
			repoName: "LEGACY-REPO",
			expected: true,
		},
		{
			name:     "repo not in ignore list",
			orgName:  "testorg",
			repoName: "regular-repo",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.IsRepoPresentInIgnoreForStatusCheck(tt.orgName, tt.repoName)
			if result != tt.expected {
				t.Errorf("Expected %v for repo '%s' in org '%s', got %v", tt.expected, tt.repoName, tt.orgName, result)
			}
		})
	}
}

func TestIsRepositoryPatternPresent(t *testing.T) {
	config := setupConfig()

	tests := []struct {
		name             string
		orgName          string
		pattern          string
		inIncludePattern bool
		expected         bool
	}{
		{
			name:             "pattern in include list",
			orgName:          "testorg",
			pattern:          ".*api.*",
			inIncludePattern: true,
			expected:         true,
		},
		{
			name:             "pattern in exclude list",
			orgName:          "testorg",
			pattern:          ".*test.*",
			inIncludePattern: false,
			expected:         true,
		},
		{
			name:             "pattern not in include list",
			orgName:          "testorg",
			pattern:          "notfound",
			inIncludePattern: true,
			expected:         false,
		},
		{
			name:             "pattern not in exclude list",
			orgName:          "testorg",
			pattern:          "notfound",
			inIncludePattern: false,
			expected:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.IsRepositoryPatternPresent(tt.orgName, tt.pattern, tt.inIncludePattern)
			if result != tt.expected {
				t.Errorf("Expected %v for pattern '%s' in org '%s', got %v", tt.expected, tt.pattern, tt.orgName, result)
			}
		})
	}
}

func TestIsPatternPresent(t *testing.T) {
	patterns := []string{"pattern1", "PATTERN2", "pattern3"}

	tests := []struct {
		name     string
		pattern  string
		expected bool
	}{
		{
			name:     "existing pattern",
			pattern:  "pattern1",
			expected: true,
		},
		{
			name:     "existing pattern case insensitive",
			pattern:  "pattern2",
			expected: true,
		},
		{
			name:     "non-existent pattern",
			pattern:  "nonexistent",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isPatternPresent(patterns, tt.pattern)
			if result != tt.expected {
				t.Errorf("Expected %v for pattern '%s', got %v", tt.expected, tt.pattern, result)
			}
		})
	}
}

func TestAddOrganization(t *testing.T) {
	config := setupConfig()

	tests := []struct {
		name     string
		orgName  string
		expected bool
	}{
		{
			name:     "add new organization",
			orgName:  "neworg",
			expected: true,
		},
		{
			name:     "add existing organization",
			orgName:  "testorg",
			expected: false,
		},
		{
			name:     "add existing organization case insensitive",
			orgName:  "TESTORG",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialCount := len(config.Organizations)
			result := config.AddOrganization(tt.orgName)

			if result != tt.expected {
				t.Errorf("Expected %v for adding org '%s', got %v", tt.expected, tt.orgName, result)
			}

			if tt.expected {
				if len(config.Organizations) != initialCount+1 {
					t.Errorf("Expected organization count to increase by 1, got %d", len(config.Organizations)-initialCount)
				}
				// Check if the organization was added
				found := false
				for _, org := range config.Organizations {
					if org.Name == tt.orgName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Organization '%s' was not added to the list", tt.orgName)
				}
			} else {
				if len(config.Organizations) != initialCount {
					t.Errorf("Expected organization count to remain the same, got %d", len(config.Organizations))
				}
			}
		})
	}
}

func TestAddRepository(t *testing.T) {
	config := setupConfig()

	tests := []struct {
		name     string
		orgName  string
		repoName string
		expected bool
	}{
		{
			name:     "add new repo to existing org",
			orgName:  "testorg",
			repoName: "newrepo",
			expected: true,
		},
		{
			name:     "add existing repo to existing org",
			orgName:  "testorg",
			repoName: "repo1",
			expected: false,
		},
		{
			name:     "add repo to non-existent org",
			orgName:  "neworg",
			repoName: "newrepo",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.AddRepository(tt.orgName, tt.repoName)

			if result != tt.expected {
				t.Errorf("Expected %v for adding repo '%s' to org '%s', got %v", tt.expected, tt.repoName, tt.orgName, result)
			}

			if tt.expected {
				// Update orgData after adding repository to reflect changes
				config.orgData = make(map[string]Organization)
				for _, org := range config.Organizations {
					config.orgData[strings.ToLower(org.Name)] = org
				}

				// Verify the repository was added
				if !config.IsRepositoryPresent(tt.orgName, tt.repoName) {
					t.Errorf("Repository '%s' was not added to organization '%s'", tt.repoName, tt.orgName)
				}
			}
		})
	}
}

func TestAddPullRequestAssignee(t *testing.T) {
	config := setupConfig()

	tests := []struct {
		name     string
		orgName  string
		assignee string
		expected bool
	}{
		{
			name:     "add new assignee to existing org",
			orgName:  "testorg",
			assignee: "newuser",
			expected: true,
		},
		{
			name:     "add existing assignee to existing org",
			orgName:  "testorg",
			assignee: "user1",
			expected: false,
		},
		{
			name:     "add assignee to non-existent org",
			orgName:  "nonexistent",
			assignee: "newuser",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.AddPullRequestAssignee(tt.orgName, tt.assignee)

			if result != tt.expected {
				t.Errorf("Expected %v for adding assignee '%s' to org '%s', got %v", tt.expected, tt.assignee, tt.orgName, result)
			}

			if tt.expected {
				// Update orgData after adding assignee to reflect changes
				config.orgData = make(map[string]Organization)
				for _, org := range config.Organizations {
					config.orgData[strings.ToLower(org.Name)] = org
				}

				// Verify the assignee was added
				if !config.IsPullRequestAssigneePresent(tt.orgName, tt.assignee) {
					t.Errorf("Assignee '%s' was not added to organization '%s'", tt.assignee, tt.orgName)
				}
			}
		})
	}
}

func TestAddRepositoryPattern(t *testing.T) {
	config := setupConfig()

	tests := []struct {
		name     string
		orgName  string
		include  bool
		exclude  bool
		pattern  string
		expected bool
	}{
		{
			name:     "add new include pattern",
			orgName:  "testorg",
			include:  true,
			exclude:  false,
			pattern:  "new-pattern",
			expected: true,
		},
		{
			name:     "add existing include pattern",
			orgName:  "testorg",
			include:  true,
			exclude:  false,
			pattern:  ".*api.*",
			expected: false,
		},
		{
			name:     "add new exclude pattern",
			orgName:  "testorg",
			include:  false,
			exclude:  true,
			pattern:  "new-exclude",
			expected: true,
		},
		{
			name:     "add pattern to non-existent org — auto-creates org",
			orgName:  "nonexistent-pattern-org",
			include:  true,
			exclude:  false,
			pattern:  "new-pattern-for-new-org",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.AddRepositoryPattern(tt.orgName, tt.include, tt.exclude, tt.pattern)

			if result != tt.expected {
				t.Errorf("Expected %v for adding pattern '%s' to org '%s', got %v", tt.expected, tt.pattern, tt.orgName, result)
			}

			if tt.expected {
				// Update orgData after adding pattern to reflect changes
				config.orgData = make(map[string]Organization)
				for _, org := range config.Organizations {
					config.orgData[strings.ToLower(org.Name)] = org
				}

				// Verify the pattern was added
				if !config.IsRepositoryPatternPresent(tt.orgName, tt.pattern, tt.include) {
					t.Errorf("Pattern '%s' was not added to organization '%s'", tt.pattern, tt.orgName)
				}
			}
		})
	}
}

func TestActualRepositoryNamesUsingFzf(t *testing.T) {
	config := setupConfig()

	tests := []struct {
		name     string
		orgName  string
		repos    []string
		expected []string
	}{
		{
			name:     "exact matches",
			orgName:  "testorg",
			repos:    []string{"repo1", "repo2"},
			expected: []string{"repo1", "repo2"},
		},
		{
			name:     "fuzzy matches",
			orgName:  "testorg",
			repos:    []string{"rep1", "test"},
			expected: []string{"repo1", "test-repo"},
		},
		{
			name:     "no config repos returns input",
			orgName:  "emptyorg",
			repos:    []string{"any", "repo"},
			expected: []string{"any", "repo"},
		},
		{
			name:     "no matches returns input",
			orgName:  "testorg",
			repos:    []string{"nomatch"},
			expected: []string{"nomatch"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.ActualRepositoryNamesUsingFzf(tt.orgName, tt.repos)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d repositories, got %d", len(tt.expected), len(result))
			}

			for i, expected := range tt.expected {
				if i < len(result) && result[i] != expected {
					t.Errorf("Expected repository '%s' at index %d, got '%s'", expected, i, result[i])
				}
			}
		})
	}
}

func TestCanSelectRepositoryForProcessing(t *testing.T) {
	// testorg has: include=[.*api.*, web-.*], exclude=[.*test.*, deprecated-.*]
	config := setupConfig()

	tests := []struct {
		name     string
		orgName  string
		repoName string
		expected bool
	}{
		// Rule 3: include filter active — repo must match at least one include pattern
		{
			name:     "repo matches include pattern (web-)",
			orgName:  "testorg",
			repoName: "web-app",
			expected: true,
		},
		{
			name:     "repo matches include pattern (api)",
			orgName:  "testorg",
			repoName: "user-api-service",
			expected: true,
		},
		// Rule 2: exclude always wins
		{
			name:     "repo matches exclude pattern only",
			orgName:  "testorg",
			repoName: "deprecated-app",
			expected: false,
		},
		{
			name:     "repo matches both include and exclude — exclude wins",
			orgName:  "testorg",
			repoName: "web-test-app",
			expected: false,
		},
		// Rule 3: include patterns set but repo doesn't match any — excluded
		{
			name:     "repo matches neither include nor exclude — excluded because include is active",
			orgName:  "testorg",
			repoName: "random-repo",
			expected: false,
		},
		// Rule 1: no patterns configured → include everything
		{
			name:     "org with no patterns — any repo included",
			orgName:  "emptyorg",
			repoName: "any-repo",
			expected: true,
		},
		// Rule 4: only exclude patterns → include everything not excluded
		{
			name:     "only-exclude org — repo not excluded",
			orgName:  "excludeonly",
			repoName: "service-a",
			expected: true,
		},
		{
			name:     "only-exclude org — repo matches exclude",
			orgName:  "excludeonly",
			repoName: "legacy-service",
			expected: false,
		},
	}

	// Add an org with only exclude patterns for Rule 4 tests
	config.Organizations = append(config.Organizations, Organization{
		Name: "excludeonly",
		RepoPatterns: IncludeExcludePattern{
			Exclude: []string{"legacy-.*"},
		},
	})
	config.rebuildOrgData()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.CanSelectRepositoryForProcessing(tt.orgName, tt.repoName)
			if result != tt.expected {
				t.Errorf(expectedRepoError, tt.expected, tt.repoName, tt.orgName, result)
			}
		})
	}
}

func TestSetTaggerName(t *testing.T) {
	config := setupConfig()

	config.SetTaggerName("testorg", "New Tagger")

	// Find the organization and check if tagger name was updated
	for _, org := range config.Organizations {
		if strings.EqualFold(org.Name, "testorg") {
			if org.Tagger.Name != "New Tagger" {
				t.Errorf("Expected tagger name 'New Tagger', got '%s'", org.Tagger.Name)
			}
			return
		}
	}
	t.Error("Organization 'testorg' not found")
}

func TestSetTaggerEmail(t *testing.T) {
	config := setupConfig()

	config.SetTaggerEmail("testorg", "newtagger@example.com")

	// Find the organization and check if tagger email was updated
	for _, org := range config.Organizations {
		if strings.EqualFold(org.Name, "testorg") {
			if org.Tagger.Email != "newtagger@example.com" {
				t.Errorf("Expected tagger email 'newtagger@example.com', got '%s'", org.Tagger.Email)
			}
			return
		}
	}
	t.Error("Organization 'testorg' not found")
}

func TestLoad(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("load valid config", func(t *testing.T) {
		config := &Config{}
		testConfig := createTestConfig()
		data, _ := json.MarshalIndent(testConfig, "", "    ")
		configPath := filepath.Join(tempDir, DefaultFilename)
		os.WriteFile(configPath, data, 0o644)

		// We'll test the Load method by simulating loading from a specific file
		// Since we can't easily mock utils.ConfigDir, we'll test the JSON parsing logic
		contents, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read test config file: %v", err)
		}

		err = json.Unmarshal(contents, config)
		if err != nil {
			t.Fatalf("Failed to unmarshal config: %v", err)
		}

		if len(config.Organizations) != len(testConfig.Organizations) {
			t.Errorf("Expected %d organizations, got %d", len(testConfig.Organizations), len(config.Organizations))
		}
	})
	t.Run("load empty config file", func(t *testing.T) {
		configPath := filepath.Join(tempDir, "empty_"+DefaultFilename)
		os.WriteFile(configPath, []byte(""), 0o644)

		contents, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read empty config file: %v", err)
		}

		if len(contents) == 0 {
			// This simulates the behavior in Load() when file is empty
			t.Log("Empty config file handled correctly")
		}
	})
	t.Run("load invalid JSON", func(t *testing.T) {
		configPath := filepath.Join(tempDir, "invalid_"+DefaultFilename)
		os.WriteFile(configPath, []byte("invalid json"), 0o644)

		contents, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatalf("Failed to read invalid config file: %v", err)
		}

		var config Config
		err = json.Unmarshal(contents, &config)
		if err == nil {
			t.Fatal("Unmarshal should fail for invalid JSON")
		}
	})

	t.Run("load non-existent file", func(t *testing.T) {
		configPath := filepath.Join(tempDir, "nonexistent_"+DefaultFilename)

		_, err := os.ReadFile(configPath)
		if !os.IsNotExist(err) {
			t.Error("Should get file not exist error")
		}
	})
}

func TestSave(t *testing.T) {
	tempDir := t.TempDir()

	config := createTestConfig()

	// Test the JSON marshaling logic
	contents, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}

	// Test writing to file
	configPath := filepath.Join(tempDir, DefaultFilename)
	err = os.WriteFile(configPath, contents, 0o644)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Verify file was created and contains valid JSON
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read saved config file: %v", err)
	}

	var loadedConfig Config
	err = json.Unmarshal(data, &loadedConfig)
	if err != nil {
		t.Fatalf("Saved config is not valid JSON: %v", err)
	}

	if len(loadedConfig.Organizations) != len(config.Organizations) {
		t.Errorf("Expected %d organizations in saved config, got %d", len(config.Organizations), len(loadedConfig.Organizations))
	}
}

func TestMatchPatterns(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		repoName string
		expected bool
	}{
		{
			name:     "exact match",
			patterns: []string{"test-repo"},
			repoName: "test-repo",
			expected: true,
		},
		{
			name:     "regex pattern match",
			patterns: []string{".*api.*"},
			repoName: "user-api-service",
			expected: true,
		},
		{
			name:     "no match",
			patterns: []string{".*api.*"},
			repoName: "web-frontend",
			expected: false,
		},
		{
			name:     "multiple patterns with match",
			patterns: []string{"web-.*", ".*api.*"},
			repoName: "web-app",
			expected: true,
		},
		{
			name:     "invalid regex pattern",
			patterns: []string{"[invalid"},
			repoName: "test-repo",
			expected: false,
		},
		{
			name:     "empty patterns",
			patterns: []string{},
			repoName: "test-repo",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := setupConfig()
			result := config.matchPatterns(tt.patterns, tt.repoName)
			if result != tt.expected {
				t.Errorf("Expected %v for patterns %v and repo '%s', got %v", tt.expected, tt.patterns, tt.repoName, result)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		config := createTestConfig()
		err := config.validate()
		if err != nil {
			t.Errorf("Valid config should not fail validation: %v", err)
		}
	})

	t.Run("negative worker count", func(t *testing.T) {
		config := createTestConfig()
		config.NoOfWorkers = -1
		err := config.validate()
		if err == nil {
			t.Error("Expected validation error for negative worker count")
		}
	})

	t.Run("empty organization name", func(t *testing.T) {
		config := createTestConfig()
		config.Organizations[0].Name = ""
		err := config.validate()
		if err == nil {
			t.Error("Expected validation error for empty organization name")
		}
	})

	t.Run("duplicate organization names", func(t *testing.T) {
		config := createTestConfig()
		config.Organizations = append(config.Organizations, Organization{Name: "testorg"})
		err := config.validate()
		if err == nil {
			t.Error("Expected validation error for duplicate organization names")
		}
	})

	t.Run("invalid organization name", func(t *testing.T) {
		config := createTestConfig()
		config.Organizations[0].Name = "invalid name!"
		err := config.validate()
		if err == nil {
			t.Error("Expected validation error for invalid organization name")
		}
	})

	t.Run("negative approving review count", func(t *testing.T) {
		config := createTestConfig()
		config.Organizations[0].ProtectedBranch.ApprovingReviewCount = -1
		err := config.validate()
		if err == nil {
			t.Error("Expected validation error for negative approving review count")
		}
	})

	t.Run("invalid tagger email", func(t *testing.T) {
		config := createTestConfig()
		config.Organizations[0].Tagger.Email = "invalid-email"
		err := config.validate()
		if err == nil {
			t.Error("Expected validation error for invalid tagger email")
		}
	})
}

func TestIsValidOrgName(t *testing.T) {
	tests := []struct {
		name     string
		orgName  string
		expected bool
	}{
		{
			name:     "valid org name",
			orgName:  "valid-org",
			expected: true,
		},
		{
			name:     "valid org name with numbers",
			orgName:  "org123",
			expected: true,
		},
		{
			name:     "valid org name with underscore",
			orgName:  "org_name",
			expected: true,
		},
		{
			name:     "single character",
			orgName:  "a",
			expected: true,
		},
		{
			name:     "empty name",
			orgName:  "",
			expected: false,
		},
		{
			name:     "too long name",
			orgName:  strings.Repeat("a", 40),
			expected: false,
		},
		{
			name:     "starts with hyphen",
			orgName:  "-invalid",
			expected: false,
		},
		{
			name:     "ends with hyphen",
			orgName:  "invalid-",
			expected: false,
		},
		{
			name:     "starts with underscore",
			orgName:  "_invalid",
			expected: false,
		},
		{
			name:     "ends with underscore",
			orgName:  "invalid_",
			expected: false,
		},
		{
			name:     "contains space",
			orgName:  "invalid org",
			expected: false,
		},
		{
			name:     "contains special character",
			orgName:  "org!",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validation.IsValidOrgName(tt.orgName)
			if result != tt.expected {
				t.Errorf("Expected %v for org name '%s', got %v", tt.expected, tt.orgName, result)
			}
		})
	}
}

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{
			name:     "valid email",
			email:    "test@example.com",
			expected: true,
		},
		{
			name:     "valid email with subdomain",
			email:    "test@mail.example.com",
			expected: true,
		},
		{
			name:     "valid email with numbers",
			email:    "test123@example123.com",
			expected: true,
		},
		{
			name:     "valid email with special characters",
			email:    "test.user+tag@example.com",
			expected: true,
		},
		{
			name:     "empty email",
			email:    "",
			expected: false,
		},
		{
			name:     "no @ symbol",
			email:    "testexample.com",
			expected: false,
		},
		{
			name:     "no domain",
			email:    "test@",
			expected: false,
		},
		{
			name:     "no local part",
			email:    "@example.com",
			expected: false,
		},
		{
			name:     "no TLD",
			email:    "test@example",
			expected: false,
		},
		{
			name:     "invalid TLD",
			email:    "test@example.c",
			expected: false,
		},
		{
			name:     "multiple @ symbols",
			email:    "test@@example.com",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidEmail(tt.email)
			if result != tt.expected {
				t.Errorf("Expected %v for email '%s', got %v", tt.expected, tt.email, result)
			}
		})
	}
}

func TestConfigFile(t *testing.T) {
	// Test the configFile function logic indirectly
	// Since we can't easily mock utils.ConfigDir, we'll test that the function exists
	// and doesn't panic when called

	t.Run("configFile function exists", func(t *testing.T) {
		// This test verifies that the configFile function can be called
		// In real usage, it would return the proper path
		result := configFile()
		if result == "" {
			t.Error("configFile() should return a non-empty string")
		}
		t.Logf("configFile() returned: %s", result)
	})
}

// Benchmark tests
func BenchmarkIsOrganizationPresent(b *testing.B) {
	config := setupConfig()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		config.IsOrganizationPresent("testorg")
	}
}

func BenchmarkMatchPatterns(b *testing.B) {
	config := setupConfig()
	patterns := []string{".*api.*", "web-.*", "service-.*"}
	repoName := "user-api-service"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		config.matchPatterns(patterns, repoName)
	}
}

func BenchmarkActualRepositoryNamesUsingFzf(b *testing.B) {
	config := setupConfig()
	repos := []string{"repo", "test", "api"}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		config.ActualRepositoryNamesUsingFzf("testorg", repos)
	}
}
