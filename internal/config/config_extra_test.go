// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// withIsolatedHome points the process's home directory at a fresh temp
// directory so tests exercising the real Init/Load/Save/configFile code
// paths never touch the developer's actual sgh config file. HOME is used
// on darwin/linux, USERPROFILE on windows — set both so the helper works
// regardless of which OS the tests run on.
func withIsolatedHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	return dir
}

// --- Init -------------------------------------------------------------

func TestInitLoadsExistingConfigFile(t *testing.T) {
	withIsolatedHome(t)

	testConfig := createTestConfig()
	data, err := json.MarshalIndent(testConfig, "", "    ")
	if err != nil {
		t.Fatalf("MarshalIndent failed: %v", err)
	}
	if err := os.WriteFile(configFile(), data, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg, err := Init()
	if err != nil {
		t.Fatalf("Init() unexpected error: %v", err)
	}
	if len(cfg.Organizations) != len(testConfig.Organizations) {
		t.Errorf("Expected %d organizations, got %d", len(testConfig.Organizations), len(cfg.Organizations))
	}
	// Init must call rebuildOrgData so lookups work immediately.
	if !cfg.IsOrganizationPresent(testOrgName) {
		t.Error("Expected orgData to be populated after Init()")
	}
}

func TestInitNoConfigFilePresent(t *testing.T) {
	withIsolatedHome(t)

	cfg, err := Init()
	if err != nil {
		t.Fatalf("Init() unexpected error when no config file exists: %v", err)
	}
	if cfg == nil {
		t.Fatal("Expected non-nil config")
	}
	if len(cfg.Organizations) != 0 {
		t.Errorf("Expected no organizations, got %d", len(cfg.Organizations))
	}
}

func TestInitInvalidJSON(t *testing.T) {
	withIsolatedHome(t)

	if err := os.WriteFile(configFile(), []byte("not valid json"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg, err := Init()
	if err == nil {
		t.Fatal("Expected error for invalid JSON config file")
	}
	if cfg != nil {
		t.Errorf("Expected nil config on error, got %+v", cfg)
	}
}

// --- Load ---------------------------------------------------------------

func TestLoadRealValidConfig(t *testing.T) {
	withIsolatedHome(t)

	testConfig := createTestConfig()
	data, _ := json.MarshalIndent(testConfig, "", "    ")
	if err := os.WriteFile(configFile(), data, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg := &Config{}
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if len(cfg.Organizations) != len(testConfig.Organizations) {
		t.Errorf("Expected %d organizations, got %d", len(testConfig.Organizations), len(cfg.Organizations))
	}
}

func TestLoadRealMissingFile(t *testing.T) {
	withIsolatedHome(t)

	cfg := &Config{}
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() should not error when config file is absent, got: %v", err)
	}
	if len(cfg.Organizations) != 0 {
		t.Errorf("Expected zero-value config, got %d organizations", len(cfg.Organizations))
	}
}

func TestLoadRealEmptyFile(t *testing.T) {
	withIsolatedHome(t)

	if err := os.WriteFile(configFile(), []byte(""), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg := &Config{}
	if err := cfg.Load(); err != nil {
		t.Fatalf("Load() should not error on empty config file, got: %v", err)
	}
	if len(cfg.Organizations) != 0 {
		t.Errorf("Expected zero-value config, got %d organizations", len(cfg.Organizations))
	}
}

func TestLoadRealInvalidJSON(t *testing.T) {
	withIsolatedHome(t)

	if err := os.WriteFile(configFile(), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg := &Config{}
	err := cfg.Load()
	if err == nil {
		t.Fatal("Load() expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse config file") {
		t.Errorf("Expected parse error message, got: %v", err)
	}
}

func TestLoadRealInvalidConfigContent(t *testing.T) {
	withIsolatedHome(t)

	// Well-formed JSON, but semantically invalid (empty org name).
	if err := os.WriteFile(configFile(), []byte(`{"organizations":[{"name":""}]}`), 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	cfg := &Config{}
	err := cfg.Load()
	if err == nil {
		t.Fatal("Load() expected error for semantically invalid config")
	}
	if !strings.Contains(err.Error(), "invalid configuration") {
		t.Errorf("Expected invalid configuration error, got: %v", err)
	}
}

func TestLoadRealReadError(t *testing.T) {
	withIsolatedHome(t)

	// Making the config path a directory forces os.ReadFile to fail with a
	// non-IsNotExist error, exercising Load()'s generic read-error branch.
	if err := os.MkdirAll(configFile(), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	cfg := &Config{}
	err := cfg.Load()
	if err == nil {
		t.Fatal("Load() expected error when config path is a directory")
	}
	if !strings.Contains(err.Error(), "failed to read config file") {
		t.Errorf("Expected read error message, got: %v", err)
	}
}

// --- Save -----------------------------------------------------------------

func TestSaveRealFile(t *testing.T) {
	withIsolatedHome(t)

	cfg := createTestConfig()
	if err := cfg.Save(); err != nil {
		t.Fatalf("Save() unexpected error: %v", err)
	}

	data, err := os.ReadFile(configFile())
	if err != nil {
		t.Fatalf("Failed to read saved config: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("Saved config is not valid JSON: %v", err)
	}
	if len(loaded.Organizations) != len(cfg.Organizations) {
		t.Errorf("Expected %d organizations, got %d", len(cfg.Organizations), len(loaded.Organizations))
	}
}

func TestSaveRealWriteError(t *testing.T) {
	withIsolatedHome(t)

	// Making the config path a pre-existing directory forces os.WriteFile
	// to fail, exercising Save()'s error-return branch.
	if err := os.MkdirAll(configFile(), 0o755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	cfg := createTestConfig()
	if err := cfg.Save(); err == nil {
		t.Fatal("Save() expected error when config path is a directory")
	}
}

// --- configFile / ConfigFilePath ------------------------------------------

func TestConfigFilePath(t *testing.T) {
	withIsolatedHome(t)

	if ConfigFilePath() != configFile() {
		t.Errorf("ConfigFilePath() = %q, want %q", ConfigFilePath(), configFile())
	}
	if ConfigFilePath() == "" {
		t.Error("ConfigFilePath() should not return an empty string")
	}
}

// --- OwnerTypeFor / SetOwnerType / TokenForOwner ---------------------------

func TestOwnerTypeForNilConfig(t *testing.T) {
	var cfg *Config
	if got := cfg.OwnerTypeFor("anything"); got != "" {
		t.Errorf("Expected empty string for nil config, got %q", got)
	}
}

func TestOwnerTypeForUninitialized(t *testing.T) {
	cfg := &Config{}
	if got := cfg.OwnerTypeFor("anything"); got != "" {
		t.Errorf("Expected empty string for config with nil orgData, got %q", got)
	}
}

func TestOwnerTypeForAndSetOwnerType(t *testing.T) {
	config := setupConfig()

	t.Run("org not present returns empty", func(t *testing.T) {
		if got := config.OwnerTypeFor(nonExistentOrg); got != "" {
			t.Errorf("Expected empty owner type, got %q", got)
		}
	})

	t.Run("set owner type on existing org", func(t *testing.T) {
		config.SetOwnerType("testorg", "Organization")
		if got := config.OwnerTypeFor("testorg"); got != "Organization" {
			t.Errorf("Expected 'Organization', got %q", got)
		}
		// Case-insensitive lookup must also reflect the update.
		if got := config.OwnerTypeFor("TESTORG"); got != "Organization" {
			t.Errorf("Expected case-insensitive lookup to return 'Organization', got %q", got)
		}
	})

	t.Run("set owner type creates new org", func(t *testing.T) {
		if config.IsOrganizationPresent("brandneworg") {
			t.Fatal("test precondition failed: org should not exist yet")
		}
		config.SetOwnerType("brandneworg", "User")
		if !config.IsOrganizationPresent("brandneworg") {
			t.Error("Expected SetOwnerType to create the organization")
		}
		if got := config.OwnerTypeFor("brandneworg"); got != "User" {
			t.Errorf("Expected 'User', got %q", got)
		}
	})
}

func TestTokenForOwner(t *testing.T) {
	config := setupConfig()

	t.Run("nil config", func(t *testing.T) {
		var nilCfg *Config
		if got := nilCfg.TokenForOwner("testorg"); got != "" {
			t.Errorf("Expected empty string for nil config, got %q", got)
		}
	})

	t.Run("org without token", func(t *testing.T) {
		if got := config.TokenForOwner("testorg"); got != "" {
			t.Errorf("Expected empty token, got %q", got)
		}
	})

	t.Run("non-existent org", func(t *testing.T) {
		if got := config.TokenForOwner(nonExistentOrg); got != "" {
			t.Errorf("Expected empty token, got %q", got)
		}
	})

	t.Run("org with token via SetToken", func(t *testing.T) {
		config.SetToken("testorg", "secret-token")
		if got := config.TokenForOwner("testorg"); got != "secret-token" {
			t.Errorf("Expected 'secret-token', got %q", got)
		}
	})

	t.Run("SetToken creates new org", func(t *testing.T) {
		config.SetToken("newtokenorg", "another-token")
		if got := config.TokenForOwner("newtokenorg"); got != "another-token" {
			t.Errorf("Expected 'another-token', got %q", got)
		}
	})
}

// --- Nil-config / not-found branches for the remaining accessors ----------

func TestAccessorsNilConfig(t *testing.T) {
	var cfg *Config

	if got := cfg.IncludePatterns("x"); len(got) != 0 {
		t.Errorf("IncludePatterns(nil) = %v, want empty", got)
	}
	if got := cfg.ExcludePatterns("x"); len(got) != 0 {
		t.Errorf("ExcludePatterns(nil) = %v, want empty", got)
	}
	if got := cfg.PullRequestAssignees("x"); len(got) != 0 {
		t.Errorf("PullRequestAssignees(nil) = %v, want empty", got)
	}
	if got := cfg.TaggerName("x"); got != "" {
		t.Errorf("TaggerName(nil) = %q, want empty", got)
	}
	if got := cfg.TaggerEmail("x"); got != "" {
		t.Errorf("TaggerEmail(nil) = %q, want empty", got)
	}
	if got := cfg.ProtectedBranchDetail("x"); got.ApprovingReviewCount != 0 || len(got.BypassPullRequestUsers) != 0 {
		t.Errorf("ProtectedBranchDetail(nil) = %+v, want zero value", got)
	}
}

func TestTaggerEmailNonExistentOrg(t *testing.T) {
	config := setupConfig()
	if got := config.TaggerEmail(nonExistentOrg); got != "" {
		t.Errorf("Expected empty tagger email for non-existent org, got %q", got)
	}
}

func TestProtectedBranchDetailNonExistentOrg(t *testing.T) {
	config := setupConfig()
	got := config.ProtectedBranchDetail(nonExistentOrg)
	if got.ApprovingReviewCount != 0 || len(got.BypassPullRequestUsers) != 0 || len(got.IgnoreBuildStatusCheckRepos) != 0 {
		t.Errorf("Expected zero-value ProtectedBranch for non-existent org, got %+v", got)
	}
}

// --- Remove* mutators -------------------------------------------------

func TestRemoveOrganization(t *testing.T) {
	tests := []struct {
		name     string
		orgName  string
		expected bool
	}{
		{"existing org", "testorg", true},
		{"existing org case insensitive", "TESTORG", true},
		{"non-existent org", nonExistentOrg, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := setupConfig()
			result := config.RemoveOrganization(tt.orgName)
			if result != tt.expected {
				t.Errorf("RemoveOrganization(%q) = %v, want %v", tt.orgName, result, tt.expected)
			}
			if tt.expected {
				if config.IsOrganizationPresent(tt.orgName) {
					t.Errorf("Expected org %q to be removed from orgData (rebuildOrgData not applied?)", tt.orgName)
				}
			}
		})
	}
}

func TestRemoveRepository(t *testing.T) {
	tests := []struct {
		name     string
		orgName  string
		repoName string
		expected bool
	}{
		{"existing repo", "testorg", "repo1", true},
		{"existing repo case insensitive", "testorg", "REPO1", true},
		{"repo not found", "testorg", "nope", false},
		{"org not found", nonExistentOrg, "repo1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := setupConfig()
			result := config.RemoveRepository(tt.orgName, tt.repoName)
			if result != tt.expected {
				t.Errorf("RemoveRepository(%q, %q) = %v, want %v", tt.orgName, tt.repoName, result, tt.expected)
			}
			if tt.expected && config.IsRepositoryPresent(tt.orgName, tt.repoName) {
				t.Errorf("Expected repo %q to be removed", tt.repoName)
			}
		})
	}
}

func TestRemovePullRequestAssignee(t *testing.T) {
	tests := []struct {
		name     string
		orgName  string
		assignee string
		expected bool
	}{
		{"existing assignee", "testorg", "user1", true},
		{"existing assignee case insensitive", "testorg", "USER1", true},
		{"assignee not found", "testorg", "nope", false},
		{"org not found", nonExistentOrg, "user1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := setupConfig()
			result := config.RemovePullRequestAssignee(tt.orgName, tt.assignee)
			if result != tt.expected {
				t.Errorf("RemovePullRequestAssignee(%q, %q) = %v, want %v", tt.orgName, tt.assignee, result, tt.expected)
			}
			if tt.expected && config.IsPullRequestAssigneePresent(tt.orgName, tt.assignee) {
				t.Errorf("Expected assignee %q to be removed", tt.assignee)
			}
		})
	}
}

func TestRemoveRepositoryPattern(t *testing.T) {
	tests := []struct {
		name     string
		orgName  string
		include  bool
		pattern  string
		expected bool
	}{
		{"include pattern found", "testorg", true, testAPIPattern, true},
		{"include pattern not found", "testorg", true, "nope", false},
		{"exclude pattern found", "testorg", false, testExcludePattern, true},
		{"exclude pattern not found", "testorg", false, "nope", false},
		{"org not found", nonExistentOrg, true, testAPIPattern, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := setupConfig()
			result := config.RemoveRepositoryPattern(tt.orgName, tt.include, tt.pattern)
			if result != tt.expected {
				t.Errorf("RemoveRepositoryPattern(%q, %v, %q) = %v, want %v", tt.orgName, tt.include, tt.pattern, result, tt.expected)
			}
			if tt.expected && config.IsRepositoryPatternPresent(tt.orgName, tt.pattern, tt.include) {
				t.Errorf("Expected pattern %q to be removed", tt.pattern)
			}
		})
	}
}

// --- AddRepositoryPattern: neither include nor exclude requested ------

func TestAddRepositoryPatternNeitherFlagSet(t *testing.T) {
	config := setupConfig()
	if got := config.AddRepositoryPattern("testorg", false, false, "irrelevant"); got != false {
		t.Errorf("AddRepositoryPattern with include=false, exclude=false = %v, want false", got)
	}
}

// --- rebuildOrgData / matchPatterns caching --------------------------

func TestRebuildOrgDataInvalidPatternsAreSkipped(t *testing.T) {
	config := setupConfig()

	if !config.AddRepositoryPattern("testorg", true, false, "[unterminated-include") {
		t.Fatal("expected invalid include pattern to still be stored on the org")
	}
	if !config.AddRepositoryPattern("testorg", false, true, "[unterminated-exclude") {
		t.Fatal("expected invalid exclude pattern to still be stored on the org")
	}

	if _, ok := config.compiledPatterns["[unterminated-include"]; ok {
		t.Error("invalid include pattern should not have been cached")
	}
	if _, ok := config.compiledPatterns["[unterminated-exclude"]; ok {
		t.Error("invalid exclude pattern should not have been cached")
	}
}

func TestMatchPatternsCacheHitAndPopulate(t *testing.T) {
	config := setupConfig()
	config.rebuildOrgData()

	// Cache hit: this pattern was already compiled by rebuildOrgData.
	if !config.matchPatterns([]string{testWebPattern}, "web-app") {
		t.Error("expected cache-hit pattern to match")
	}

	// Cache miss but compiledPatterns is non-nil: pattern gets compiled
	// and stored for reuse.
	newPattern := "^brand-new-uncached-.*"
	if !config.matchPatterns([]string{newPattern}, "brand-new-uncached-repo") {
		t.Error("expected newly compiled pattern to match")
	}
	if _, ok := config.compiledPatterns[newPattern]; !ok {
		t.Error("expected newly compiled pattern to be cached")
	}
}

// --- resolveRepoName ----------------------------------------------------

func TestResolveRepoName(t *testing.T) {
	t.Run("exact case-insensitive match", func(t *testing.T) {
		got := resolveRepoName("REPO1", []string{"repo1", "repo2"})
		if got != "repo1" {
			t.Errorf("resolveRepoName() = %q, want %q", got, "repo1")
		}
	})

	t.Run("single fuzzy match", func(t *testing.T) {
		got := resolveRepoName("singlematc", []string{"singlematch", "totally-different"})
		if got != "singlematch" {
			t.Errorf("resolveRepoName() = %q, want %q", got, "singlematch")
		}
	})

	t.Run("multiple fuzzy matches picks a ranked candidate", func(t *testing.T) {
		configNames := []string{"testalpha", "testbeta"}
		got := resolveRepoName("test", configNames)
		found := false
		for _, n := range configNames {
			if got == n {
				found = true
			}
		}
		if !found {
			t.Errorf("resolveRepoName() = %q, want one of %v", got, configNames)
		}
	})

	t.Run("no match returns input unchanged", func(t *testing.T) {
		got := resolveRepoName("zzzzzzzzzzzzzzzzzzzzz", []string{"repo1", "repo2"})
		if got != "zzzzzzzzzzzzzzzzzzzzz" {
			t.Errorf("resolveRepoName() = %q, want input echoed back", got)
		}
	})
}

// --- Validate / validate ------------------------------------------------

func TestValidateExported(t *testing.T) {
	config := createTestConfig()
	if err := config.Validate(); err != nil {
		t.Errorf("Validate() unexpected error: %v", err)
	}

	config.NoOfWorkers = -1
	if err := config.Validate(); err == nil {
		t.Error("Validate() expected error for negative worker count")
	}
}

func TestValidateAdditionalBranches(t *testing.T) {
	t.Run("high worker count only warns", func(t *testing.T) {
		config := createTestConfig()
		config.NoOfWorkers = 150
		if err := config.validate(); err != nil {
			t.Errorf("High worker count should only warn, got error: %v", err)
		}
	})

	t.Run("high approving review count only warns", func(t *testing.T) {
		config := createTestConfig()
		config.Organizations[0].ProtectedBranch.ApprovingReviewCount = 20
		if err := config.validate(); err != nil {
			t.Errorf("High approving review count should only warn, got error: %v", err)
		}
	})

	t.Run("invalid repo pattern", func(t *testing.T) {
		config := createTestConfig()
		config.Organizations[0].RepoPatterns.Include = []string{"[invalid"}
		err := config.validate()
		if err == nil {
			t.Fatal("Expected validation error for invalid repo pattern")
		}
		if !strings.Contains(err.Error(), "invalid repository patterns") {
			t.Errorf("Expected repository patterns error, got: %v", err)
		}
	})

	t.Run("invalid pull request assignee", func(t *testing.T) {
		config := createTestConfig()
		config.Organizations[0].PullRequestAssignees = []string{"bad--name"}
		err := config.validate()
		if err == nil {
			t.Fatal("Expected validation error for invalid PR assignee")
		}
		if !strings.Contains(err.Error(), "invalid pull request assignees") {
			t.Errorf("Expected pull request assignees error, got: %v", err)
		}
	})

	t.Run("invalid bypass pull request user", func(t *testing.T) {
		config := createTestConfig()
		config.Organizations[0].ProtectedBranch.BypassPullRequestUsers = []string{""}
		err := config.validate()
		if err == nil {
			t.Fatal("Expected validation error for invalid bypass user")
		}
		if !strings.Contains(err.Error(), "invalid bypass users") {
			t.Errorf("Expected bypass users error, got: %v", err)
		}
	})

	t.Run("invalid allowed restrictions user", func(t *testing.T) {
		config := createTestConfig()
		config.Organizations[0].ProtectedBranch.AllowedRestrictionsUsers = []string{"-leading-hyphen"}
		err := config.validate()
		if err == nil {
			t.Fatal("Expected validation error for invalid allowed restrictions user")
		}
		if !strings.Contains(err.Error(), "invalid allowed restrictions users") {
			t.Errorf("Expected allowed restrictions users error, got: %v", err)
		}
	})
}

// --- ValidatePattern ------------------------------------------------------

func TestValidatePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{"empty pattern", "", true},
		{"whitespace-only pattern", "   ", true},
		{"path traversal", "..", true},
		{"path traversal in path-like pattern", "../etc/passwd", true},
		{"literal catch-all", ".*", true},
		{"literal catch-all star", "*", true},
		{"invalid regex", "[invalid", true},
		{"runtime catch-all not in literal list", "(?:.*)", true},
		{"valid specific pattern", "^my-service-.*", false},
		{"valid literal pattern", "test-repo", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePattern(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePattern(%q) error = %v, wantErr %v", tt.pattern, err, tt.wantErr)
			}
		})
	}
}

// --- validateRepoPatterns -------------------------------------------------

func TestValidateRepoPatterns(t *testing.T) {
	t.Run("valid patterns", func(t *testing.T) {
		patterns := IncludeExcludePattern{
			Include: []string{"^service-"},
			Exclude: []string{"^legacy-"},
		}
		if err := validateRepoPatterns(patterns); err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("invalid include pattern", func(t *testing.T) {
		patterns := IncludeExcludePattern{Include: []string{"[bad"}}
		err := validateRepoPatterns(patterns)
		if err == nil || !strings.Contains(err.Error(), "include pattern") {
			t.Errorf("Expected include pattern error, got: %v", err)
		}
	})

	t.Run("invalid exclude pattern", func(t *testing.T) {
		patterns := IncludeExcludePattern{
			Include: []string{"^service-"},
			Exclude: []string{"[bad"},
		}
		err := validateRepoPatterns(patterns)
		if err == nil || !strings.Contains(err.Error(), "exclude pattern") {
			t.Errorf("Expected exclude pattern error, got: %v", err)
		}
	})
}

// --- validateUserList / isValidGitHubUsername -----------------------------

func TestValidateUserList(t *testing.T) {
	t.Run("valid list", func(t *testing.T) {
		if err := validateUserList([]string{"alice", "bob-smith"}, "field"); err != nil {
			t.Errorf("Expected no error, got: %v", err)
		}
	})

	t.Run("empty username", func(t *testing.T) {
		err := validateUserList([]string{"alice", ""}, "field")
		if err == nil || !strings.Contains(err.Error(), "empty user name") {
			t.Errorf("Expected empty user name error, got: %v", err)
		}
	})

	t.Run("invalid username", func(t *testing.T) {
		err := validateUserList([]string{"bad--name"}, "field")
		if err == nil || !strings.Contains(err.Error(), "invalid GitHub username") {
			t.Errorf("Expected invalid username error, got: %v", err)
		}
	})
}

func TestIsValidGitHubUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		expected bool
	}{
		{"valid simple", "alice", true},
		{"valid with hyphen", "alice-bob", true},
		{"empty", "", false},
		{"too long", strings.Repeat("a", 40), false},
		{"starts with hyphen", "-alice", false},
		{"ends with hyphen", "alice-", false},
		{"consecutive hyphens", "al--ice", false},
		{"max length boundary", strings.Repeat("a", 39), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidGitHubUsername(tt.username)
			if result != tt.expected {
				t.Errorf("isValidGitHubUsername(%q) = %v, want %v", tt.username, result, tt.expected)
			}
		})
	}
}
