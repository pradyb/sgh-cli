// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package pr

import (
	"testing"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/service/servicetest"
	"github.com/pradyb/sgh-cli/internal/testutils"
)

func TestParsePatchLines(t *testing.T) {
	tests := []struct {
		name  string
		files []model.PullRequestFile
		want  []string
	}{
		{
			name:  "empty patch is skipped",
			files: []model.PullRequestFile{{Filename: "empty.go", Patch: ""}},
			want:  nil,
		},
		{
			name: "hunk header is trimmed after the closing @@ marker",
			files: []model.PullRequestFile{
				{Filename: "a.go", Patch: "@@ -1,3 +1,4 @@ func foo() {\n+line\n-oldline\n context line\n\n"},
			},
			want: []string{
				"── a.go ──",
				"@@ -1,3 +1,4 @@",
				"+line",
				"-oldline",
				" context line",
			},
		},
		{
			name: "hunk header without a closing marker is kept as-is",
			files: []model.PullRequestFile{
				{Filename: "b.go", Patch: "@@no-closing-marker\n+added"},
			},
			want: []string{
				"── b.go ──",
				"@@no-closing-marker",
				"+added",
			},
		},
		{
			name: "multiple files are each headered, empty ones skipped",
			files: []model.PullRequestFile{
				{Filename: "skip.go", Patch: ""},
				{Filename: "c.go", Patch: "+only line"},
			},
			want: []string{
				"── c.go ──",
				"+only line",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParsePatchLines(model.PullRequestFilesResponse{Files: tt.files})
			if len(got) != len(tt.want) {
				t.Fatalf("ParsePatchLines() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMatchesPRFilters(t *testing.T) {
	basePR := model.PullRequestResponse{
		Author:    model.User{Login: "alice"},
		Assignees: []model.User{{Login: "bob"}},
		Reviewers: []model.Actor{
			{Type: "User", User: model.User{Login: "carol"}},
			{Type: "Team", Team: model.OrgTeam{Name: "core-team"}},
		},
		Labels:    []string{"bug", "urgent"},
		CreatedAt: "2024-06-15",
	}

	tests := []struct {
		name string
		req  PRRequest
		want bool
	}{
		{name: "no filters matches everything", req: PRRequest{}, want: true},
		{name: "author matches case-insensitively", req: PRRequest{Author: "ALICE"}, want: true},
		{name: "author mismatch excludes", req: PRRequest{Author: "dave"}, want: false},
		{name: "assignee found", req: PRRequest{Assignee: "Bob"}, want: true},
		{name: "assignee not found", req: PRRequest{Assignee: "erin"}, want: false},
		{name: "reviewer user match", req: PRRequest{Reviewer: "carol"}, want: true},
		{name: "reviewer team match", req: PRRequest{Reviewer: "core-team"}, want: true},
		{name: "reviewer no match", req: PRRequest{Reviewer: "frank"}, want: false},
		{name: "label match", req: PRRequest{Label: "URGENT"}, want: true},
		{name: "label no match", req: PRRequest{Label: "wontfix"}, want: false},
		{name: "since before created date passes", req: PRRequest{Since: "2024-01-01"}, want: true},
		{name: "since after created date excludes", req: PRRequest{Since: "2024-12-01"}, want: false},
		{
			name: "all filters combined match",
			req:  PRRequest{Author: "alice", Assignee: "bob", Reviewer: "carol", Label: "bug", Since: "2024-01-01"},
			want: true,
		},
		{
			name: "all filters combined, one mismatching label",
			req:  PRRequest{Author: "alice", Assignee: "bob", Reviewer: "carol", Label: "nope"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesPRFilters(basePR, tt.req); got != tt.want {
				t.Errorf("matchesPRFilters() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetSearchQuery(t *testing.T) {
	mockServer := testutils.NewMockGitHubServer()
	defer mockServer.Close()
	ctx := servicetest.NewMockContext(t, mockServer)

	tests := []struct {
		name string
		req  PRRequest
		want string
	}{
		{
			name: "org scope, open PRs by default",
			req:  PRRequest{OrgName: "sample-org"},
			want: "org:sample-org type:pr state:open sort:created-desc",
		},
		{
			name: "org scope, all PRs when All is set",
			req:  PRRequest{OrgName: "sample-org", All: true},
			want: "org:sample-org type:pr sort:created-desc",
		},
		{
			name: "single repo scope",
			req:  PRRequest{OrgName: "sample-org", RepoNames: []string{"my-repo"}},
			want: "repo:sample-org/my-repo type:pr state:open sort:created-desc",
		},
		{
			name: "all filters appended in order",
			req: PRRequest{
				OrgName:  "sample-org",
				BaseRef:  "main",
				HeadRef:  "feature",
				Author:   "jane",
				Assignee: "john",
				Reviewer: "team-x",
				Label:    "bug",
				Since:    "2024-01-01",
			},
			want: "org:sample-org type:pr state:open sort:created-desc base:main head:feature author:jane assignee:john review-requested:team-x label:bug created:>=2024-01-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getSearchQuery(ctx, tt.req); got != tt.want {
				t.Errorf("getSearchQuery() =\n  %q\nwant\n  %q", got, tt.want)
			}
		})
	}
}
