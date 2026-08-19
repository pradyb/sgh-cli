// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package ui

import (
	"testing"

	"github.com/pradyb/sgh-cli/internal/model"
)

// The Sort* helpers all mutate in place and all silently ignore an unknown
// sortBy. That combination makes wrong output easy to ship unnoticed, which is
// why each one is pinned here.

func branchNames(branches []model.BranchResponse) []string {
	out := make([]string, len(branches))
	for i, b := range branches {
		out[i] = b.Name
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSortBranches(t *testing.T) {
	newSet := func() []model.BranchResponse {
		return []model.BranchResponse{
			{Name: "main", RepositoryName: "web", Protected: false},
			{Name: "develop", RepositoryName: "api", Protected: true},
			{Name: "feature", RepositoryName: "service", Protected: false},
		}
	}

	t.Run("by repo", func(t *testing.T) {
		got := newSet()
		SortBranches(got, "repo")
		// api, service, web
		if want := []string{"develop", "feature", "main"}; !equal(branchNames(got), want) {
			t.Errorf("got %v, want %v", branchNames(got), want)
		}
	})

	t.Run("by name", func(t *testing.T) {
		got := newSet()
		SortBranches(got, "name")
		if want := []string{"develop", "feature", "main"}; !equal(branchNames(got), want) {
			t.Errorf("got %v, want %v", branchNames(got), want)
		}
	})

	t.Run("by protected puts protected first", func(t *testing.T) {
		got := newSet()
		SortBranches(got, "protected")
		if !got[0].Protected {
			t.Errorf("first entry = %+v, want a protected branch", got[0])
		}
	})

	t.Run("sortBy is case-insensitive", func(t *testing.T) {
		got := newSet()
		SortBranches(got, "NAME")
		if want := []string{"develop", "feature", "main"}; !equal(branchNames(got), want) {
			t.Errorf("got %v, want %v", branchNames(got), want)
		}
	})

	t.Run("unknown key leaves order untouched", func(t *testing.T) {
		got := newSet()
		SortBranches(got, "nonsense")
		if want := []string{"main", "develop", "feature"}; !equal(branchNames(got), want) {
			t.Errorf("got %v, want the original order %v", branchNames(got), want)
		}
	})

	t.Run("empty slice does not panic", func(t *testing.T) {
		SortBranches(nil, "name")
		SortBranches([]model.BranchResponse{}, "repo")
	})
}

func TestSortTags(t *testing.T) {
	newSet := func() []model.TagResponse {
		return []model.TagResponse{
			{Name: "v2.0.0", RepositoryName: "web"},
			{Name: "v1.0.0", RepositoryName: "api"},
		}
	}

	t.Run("by tag", func(t *testing.T) {
		got := newSet()
		SortTags(got, "tag")
		if got[0].Name != "v1.0.0" {
			t.Errorf("first = %q, want v1.0.0", got[0].Name)
		}
	})

	t.Run("by repo", func(t *testing.T) {
		got := newSet()
		SortTags(got, "repo")
		if got[0].RepositoryName != "api" {
			t.Errorf("first repo = %q, want api", got[0].RepositoryName)
		}
	})

	t.Run("unknown key leaves order untouched", func(t *testing.T) {
		got := newSet()
		SortTags(got, "nope")
		if got[0].Name != "v2.0.0" {
			t.Errorf("order changed unexpectedly: first = %q", got[0].Name)
		}
	})

	t.Run("empty slice does not panic", func(t *testing.T) {
		SortTags(nil, "tag")
	})
}

func TestSortCommits(t *testing.T) {
	newSet := func() []model.CommitResponse {
		var a, b, c model.CommitResponse

		a.RepositoryName = "web"
		a.Commit.Author.Name = "Zoe"
		a.Commit.Author.Date = "2026-01-01T00:00:00Z"

		b.RepositoryName = "api"
		b.Commit.Author.Name = "Alice"
		b.Commit.Author.Date = "2026-08-01T00:00:00Z"

		c.RepositoryName = "service"
		c.Commit.Author.Name = "Mo"
		c.Commit.Author.Date = "2026-04-01T00:00:00Z"

		return []model.CommitResponse{a, b, c}
	}

	t.Run("by repo", func(t *testing.T) {
		got := newSet()
		SortCommits(got, "repo")
		if got[0].RepositoryName != "api" {
			t.Errorf("first repo = %q, want api", got[0].RepositoryName)
		}
	})

	t.Run("by author", func(t *testing.T) {
		got := newSet()
		SortCommits(got, "author")
		if got[0].Commit.Author.Name != "Alice" {
			t.Errorf("first author = %q, want Alice", got[0].Commit.Author.Name)
		}
	})

	// Unlike every other Sort* helper, commits default to newest-first rather
	// than leaving the order alone.
	t.Run("default is newest first", func(t *testing.T) {
		got := newSet()
		SortCommits(got, "")
		if got[0].Commit.Author.Date != "2026-08-01T00:00:00Z" {
			t.Errorf("first date = %q, want the newest", got[0].Commit.Author.Date)
		}
	})

	t.Run("unknown key also sorts by date", func(t *testing.T) {
		got := newSet()
		SortCommits(got, "nonsense")
		if got[0].Commit.Author.Date != "2026-08-01T00:00:00Z" {
			t.Errorf("first date = %q, want the newest", got[0].Commit.Author.Date)
		}
	})

	t.Run("unparseable dates do not panic", func(t *testing.T) {
		var a, b model.CommitResponse
		a.Commit.Author.Date = "not-a-date"
		b.Commit.Author.Date = ""
		SortCommits([]model.CommitResponse{a, b}, "date")
	})
}

func TestSortWorkflowRuns(t *testing.T) {
	newSet := func() []model.WorkflowRun {
		return []model.WorkflowRun{
			{Name: "Release", RepositoryName: "web", Status: "completed", Conclusion: "success", CreatedAt: "2026-01-01T00:00:00Z"},
			{Name: "CI", RepositoryName: "api", Status: "in_progress", Conclusion: "", CreatedAt: "2026-08-01T00:00:00Z"},
		}
	}

	t.Run("by name", func(t *testing.T) {
		got := newSet()
		SortWorkflowRuns(got, "name")
		if got[0].Name != "CI" {
			t.Errorf("first = %q, want CI", got[0].Name)
		}
	})

	t.Run("by repo", func(t *testing.T) {
		got := newSet()
		SortWorkflowRuns(got, "repo")
		if got[0].RepositoryName != "api" {
			t.Errorf("first repo = %q, want api", got[0].RepositoryName)
		}
	})

	// A run still in progress has no conclusion, so status stands in for it.
	t.Run("status falls back to Status when Conclusion is empty", func(t *testing.T) {
		got := newSet()
		SortWorkflowRuns(got, "status")
		if got[0].Conclusion != "" || got[0].Status != "in_progress" {
			t.Errorf("first = %+v, want the in_progress run sorted on its status", got[0])
		}
	})

	t.Run("by created is newest first", func(t *testing.T) {
		got := newSet()
		SortWorkflowRuns(got, "created")
		if got[0].CreatedAt != "2026-08-01T00:00:00Z" {
			t.Errorf("first created = %q, want the newest", got[0].CreatedAt)
		}
	})

	t.Run("unknown key leaves order untouched", func(t *testing.T) {
		got := newSet()
		SortWorkflowRuns(got, "nope")
		if got[0].Name != "Release" {
			t.Errorf("order changed unexpectedly: first = %q", got[0].Name)
		}
	})

	t.Run("empty slice does not panic", func(t *testing.T) {
		SortWorkflowRuns(nil, "status")
	})
}

func TestSortSecretAlerts(t *testing.T) {
	newSet := func() []model.SecretScanningAlert {
		return []model.SecretScanningAlert{
			{Number: 1, RepositoryName: "web", State: "resolved", SecretType: "slack_token", CreatedAt: "2026-01-01T00:00:00Z"},
			{Number: 2, RepositoryName: "api", State: "open", SecretType: "aws_access_key", CreatedAt: "2026-08-01T00:00:00Z"},
		}
	}

	t.Run("by repo", func(t *testing.T) {
		got := newSet()
		SortSecretAlerts(got, "repo")
		if got[0].RepositoryName != "api" {
			t.Errorf("first repo = %q, want api", got[0].RepositoryName)
		}
	})

	t.Run("by state", func(t *testing.T) {
		got := newSet()
		SortSecretAlerts(got, "state")
		if got[0].State != "open" {
			t.Errorf("first state = %q, want open", got[0].State)
		}
	})

	t.Run("by type", func(t *testing.T) {
		got := newSet()
		SortSecretAlerts(got, "type")
		if got[0].SecretType != "aws_access_key" {
			t.Errorf("first type = %q, want aws_access_key", got[0].SecretType)
		}
	})

	t.Run("by created is newest first", func(t *testing.T) {
		got := newSet()
		SortSecretAlerts(got, "created")
		if got[0].CreatedAt != "2026-08-01T00:00:00Z" {
			t.Errorf("first created = %q, want the newest", got[0].CreatedAt)
		}
	})

	t.Run("unknown key leaves order untouched", func(t *testing.T) {
		got := newSet()
		SortSecretAlerts(got, "nope")
		if got[0].Number != 1 {
			t.Errorf("order changed unexpectedly: first = %d", got[0].Number)
		}
	})

	t.Run("empty slice does not panic", func(t *testing.T) {
		SortSecretAlerts(nil, "state")
	})
}
