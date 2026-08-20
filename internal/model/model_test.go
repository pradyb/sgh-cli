// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package model

import (
	"encoding/json"
	"testing"
)

// ---- PullRequestResponse ----

func TestPullRequestResponse_RepositoryName(t *testing.T) {
	pr := PullRequestResponse{Base: PRBranch{Repo: Repository{Name: "my-repo"}}}
	if got := pr.RepositoryName(); got != "my-repo" {
		t.Errorf("RepositoryName() = %q, want my-repo", got)
	}
}

func TestPullRequestResponse_AuthorName(t *testing.T) {
	tests := []struct {
		name string
		pr   PullRequestResponse
		want string
	}{
		{"name set", PullRequestResponse{Author: User{Name: "Jane Doe", Login: "janedoe"}}, "Jane Doe"},
		{"name empty falls back to login", PullRequestResponse{Author: User{Login: "janedoe"}}, "janedoe"},
		{"both empty", PullRequestResponse{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pr.AuthorName(); got != tt.want {
				t.Errorf("AuthorName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPullRequestResponse_AssigneesName(t *testing.T) {
	tests := []struct {
		name string
		pr   PullRequestResponse
		want string
	}{
		{"nil", PullRequestResponse{}, ""},
		{"empty slice", PullRequestResponse{Assignees: []User{}}, ""},
		{"single with name", PullRequestResponse{Assignees: []User{{Name: "Alice"}}}, "Alice"},
		{"single with login only", PullRequestResponse{Assignees: []User{{Login: "bob"}}}, "bob"},
		{"multiple mixed with fully empty entry", PullRequestResponse{Assignees: []User{{Name: "Alice"}, {Login: "bob"}, {}}}, "Alice\nbob"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pr.AssigneesName(); got != tt.want {
				t.Errorf("AssigneesName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPullRequestResponse_ReviewersName(t *testing.T) {
	tests := []struct {
		name string
		pr   PullRequestResponse
		want string
	}{
		{"nil", PullRequestResponse{}, ""},
		{"user reviewer with name", PullRequestResponse{Reviewers: []Actor{{Type: "User", User: User{Name: "Carl"}}}}, "Carl"},
		{"user reviewer login only", PullRequestResponse{Reviewers: []Actor{{Type: "User", User: User{Login: "carl"}}}}, "carl"},
		{"team reviewer", PullRequestResponse{Reviewers: []Actor{{Type: "Team", Team: OrgTeam{Name: "core-team"}}}}, "core-team"},
		{
			"multiple reviewers joined",
			PullRequestResponse{Reviewers: []Actor{
				{Type: "User", User: User{Name: "Carl"}},
				{Type: "Team", Team: OrgTeam{Name: "core-team"}},
			}},
			"Carl\ncore-team",
		},
		{"reviewer with no name at all is skipped", PullRequestResponse{Reviewers: []Actor{{Type: "User"}}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pr.ReviewersName(); got != tt.want {
				t.Errorf("ReviewersName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPullRequestResponse_FirstReviewerName(t *testing.T) {
	tests := []struct {
		name string
		pr   PullRequestResponse
		want string
	}{
		{"no reviewers", PullRequestResponse{}, ""},
		{"single reviewer with name", PullRequestResponse{Reviewers: []Actor{{Type: "User", User: User{Name: "Carl"}}}}, "Carl"},
		{"single reviewer login only", PullRequestResponse{Reviewers: []Actor{{Type: "User", User: User{Login: "carl"}}}}, "carl"},
		{
			"multiple reviewers appends ellipsis",
			PullRequestResponse{Reviewers: []Actor{
				{Type: "User", User: User{Name: "Carl"}},
				{Type: "User", User: User{Name: "Dana"}},
			}},
			"Carl...",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pr.FirstReviewerName(); got != tt.want {
				t.Errorf("FirstReviewerName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPullRequestResponse_StateIcon(t *testing.T) {
	tests := []struct {
		state string
		want  string
	}{
		{"open", "●"},
		{"OPEN", "●"},
		{"closed", "✗"},
		{"CLOSED", "✗"},
		{"merged", "⊕"},
		{"MERGED", "⊕"},
		{"", "·"},
		{"draft", "·"},
	}
	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			pr := PullRequestResponse{State: tt.state}
			if got := pr.stateIcon(); got != tt.want {
				t.Errorf("stateIcon() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPullRequestResponse_Title(t *testing.T) {
	pr := PullRequestResponse{
		State:     "open",
		PRNumber:  42,
		TitleName: "Fix bug",
		Base:      PRBranch{Repo: Repository{Name: "my-repo"}},
	}
	want := "● #42 Fix bug (my-repo)"
	if got := pr.Title(); got != want {
		t.Errorf("Title() = %q, want %q", got, want)
	}
}

func TestPullRequestResponse_Description(t *testing.T) {
	tests := []struct {
		name string
		pr   PullRequestResponse
		want string
	}{
		{
			"no reviewer",
			PullRequestResponse{
				State:  "open",
				Author: User{Name: "Jane"},
				Base:   PRBranch{Ref: "main"},
				Head:   PRBranch{Ref: "feature"},
			},
			"[open] Jane  main ← feature",
		},
		{
			"with reviewer",
			PullRequestResponse{
				State:     "open",
				Author:    User{Name: "Jane"},
				Base:      PRBranch{Ref: "main"},
				Head:      PRBranch{Ref: "feature"},
				Reviewers: []Actor{{Type: "User", User: User{Name: "Carl"}}},
			},
			"[open] Jane → Carl  main ← feature",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.pr.Description(); got != tt.want {
				t.Errorf("Description() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPullRequestResponse_FilterValue(t *testing.T) {
	pr := PullRequestResponse{
		State:     "open",
		PRNumber:  1,
		TitleName: "Test",
		Base:      PRBranch{Repo: Repository{Name: "repo"}},
	}
	if got := pr.FilterValue(); got != pr.Title() {
		t.Errorf("FilterValue() = %q, want %q", got, pr.Title())
	}
}

// ---- Actor ----

func TestActor_Name(t *testing.T) {
	tests := []struct {
		name  string
		actor Actor
		want  string
	}{
		{"user actor", Actor{Type: "User", User: User{Name: "Jane"}}, "Jane"},
		{"team actor", Actor{Type: "Team", Team: OrgTeam{Name: "core"}}, "core"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.actor.Name(); got != tt.want {
				t.Errorf("Name() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestActor_Login(t *testing.T) {
	tests := []struct {
		name  string
		actor Actor
		want  string
	}{
		{"user actor", Actor{Type: "User", User: User{Login: "janedoe"}}, "janedoe"},
		{"team actor uses team name", Actor{Type: "Team", Team: OrgTeam{Name: "core"}}, "core"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.actor.Login(); got != tt.want {
				t.Errorf("Login() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---- RefUIResponse / CreateNewCommonResponse ----

func TestRefUIResponse_IsSuccess(t *testing.T) {
	tests := []struct {
		name string
		resp RefUIResponse
		want bool
	}{
		{"no error message", RefUIResponse{ErrorMessage: ""}, true},
		{"error message set", RefUIResponse{ErrorMessage: "boom"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resp.IsSuccess(); got != tt.want {
				t.Errorf("IsSuccess() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateNewCommonResponse_FieldOrder(t *testing.T) {
	// Verify each argument lands in its documented field - this constructor's
	// parameter order (repoName, ref, refType, successMessage, errorMessage)
	// has historically been a source of bugs elsewhere in this codebase
	// (pkg/branch), so pin every field to a distinct value to catch any
	// transposition.
	got := CreateNewCommonResponse("repo-name", "ref-value", "REF_TYPE", "success-msg", "error-msg")

	want := RefUIResponse{
		RepositoryName: "repo-name",
		Ref:            "ref-value",
		Type:           "REF_TYPE",
		SuccessMessage: "success-msg",
		ErrorMessage:   "error-msg",
	}
	if got != want {
		t.Errorf("CreateNewCommonResponse() = %+v, want %+v", got, want)
	}
}

func TestCreateNewCommonResponse_SuccessCase(t *testing.T) {
	got := CreateNewCommonResponse("repo", "main", "CREATE_BRANCH", "Branch created", "")
	if !got.IsSuccess() {
		t.Errorf("expected success response, got %+v", got)
	}
	if got.SuccessMessage != "Branch created" {
		t.Errorf("SuccessMessage = %q, want %q", got.SuccessMessage, "Branch created")
	}
}

func TestCreateNewCommonResponse_ErrorCase(t *testing.T) {
	got := CreateNewCommonResponse("repo", "main", "CREATE_BRANCH", "", "failed to create branch")
	if got.IsSuccess() {
		t.Errorf("expected failure response, got %+v", got)
	}
	if got.ErrorMessage != "failed to create branch" {
		t.Errorf("ErrorMessage = %q, want %q", got.ErrorMessage, "failed to create branch")
	}
}

// ---- CommitResponse ----

func TestCommitResponse_RepoName(t *testing.T) {
	cr := CommitResponse{RepositoryName: "my-repo"}
	if got := cr.RepoName(); got != "my-repo" {
		t.Errorf("RepoName() = %q, want my-repo", got)
	}
}

// ---- WorkflowRunDetail ----

func TestWorkflowRunDetail_IsInProgress(t *testing.T) {
	tests := []struct {
		name string
		d    WorkflowRunDetail
		want bool
	}{
		{"queued", WorkflowRunDetail{Run: WorkflowRun{Status: "queued"}}, true},
		{"in_progress", WorkflowRunDetail{Run: WorkflowRun{Status: "in_progress"}}, true},
		{"waiting", WorkflowRunDetail{Run: WorkflowRun{Status: "waiting"}}, true},
		{"requested", WorkflowRunDetail{Run: WorkflowRun{Status: "requested"}}, true},
		{"pending", WorkflowRunDetail{Run: WorkflowRun{Status: "pending"}}, true},
		{"completed", WorkflowRunDetail{Run: WorkflowRun{Status: "completed"}}, false},
		{"unknown status", WorkflowRunDetail{Run: WorkflowRun{Status: "cancelled"}}, false},
		{"error message present overrides in-progress status", WorkflowRunDetail{Run: WorkflowRun{Status: "queued"}, ErrorMessage: "boom"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.d.IsInProgress(); got != tt.want {
				t.Errorf("IsInProgress() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---- IssueResponse ----

func TestIssueResponse_AuthorName(t *testing.T) {
	tests := []struct {
		name  string
		issue IssueResponse
		want  string
	}{
		{"name set", IssueResponse{Author: User{Name: "Jane Doe", Login: "janedoe"}}, "Jane Doe"},
		{"falls back to login", IssueResponse{Author: User{Login: "janedoe"}}, "janedoe"},
		{"both empty", IssueResponse{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.issue.AuthorName(); got != tt.want {
				t.Errorf("AuthorName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIssueResponse_LabelNames(t *testing.T) {
	tests := []struct {
		name  string
		issue IssueResponse
		want  string
	}{
		{"no labels", IssueResponse{}, ""},
		{"empty slice", IssueResponse{Labels: []IssueLabel{}}, ""},
		{"single label", IssueResponse{Labels: []IssueLabel{{Name: "bug"}}}, "bug"},
		{"multiple labels", IssueResponse{Labels: []IssueLabel{{Name: "bug"}, {Name: "urgent"}, {Name: "help wanted"}}}, "bug, urgent, help wanted"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.issue.LabelNames(); got != tt.want {
				t.Errorf("LabelNames() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIssueResponse_IsIssue(t *testing.T) {
	tests := []struct {
		name  string
		issue IssueResponse
		want  bool
	}{
		{"no pull request field", IssueResponse{PullRequest: nil}, true},
		{"pull request field present", IssueResponse{PullRequest: &IssuePR{URL: "https://example.com/pr/1"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.issue.IsIssue(); got != tt.want {
				t.Errorf("IsIssue() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---- AuditLogEntry.UnmarshalJSON ----

func TestAuditLogEntry_UnmarshalJSON_StringRepo(t *testing.T) {
	data := `{"action":"repo.create","actor":"alice","actor_ip":"1.2.3.4","created_at":1700000000000,"org":"myorg","repo":"myorg/repo1","user":"bob"}`
	var e AuditLogEntry
	if err := json.Unmarshal([]byte(data), &e); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Repo != "myorg/repo1" {
		t.Errorf("Repo = %q, want myorg/repo1", e.Repo)
	}
	if e.Action != "repo.create" || e.Actor != "alice" || e.ActorIP != "1.2.3.4" || e.OrgName != "myorg" || e.User != "bob" {
		t.Errorf("unexpected field mapping: %+v", e)
	}
	if e.CreatedAt != 1700000000000 {
		t.Errorf("CreatedAt = %d, want 1700000000000", e.CreatedAt)
	}
}

func TestAuditLogEntry_UnmarshalJSON_ArrayRepo(t *testing.T) {
	data := `{"action":"repo.access","repo":["myorg/repo1","myorg/repo2"]}`
	var e AuditLogEntry
	if err := json.Unmarshal([]byte(data), &e); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Repo != "myorg/repo1" {
		t.Errorf("Repo = %q, want first element myorg/repo1", e.Repo)
	}
}

func TestAuditLogEntry_UnmarshalJSON_EmptyArrayRepo(t *testing.T) {
	data := `{"action":"org.update","repo":[]}`
	var e AuditLogEntry
	if err := json.Unmarshal([]byte(data), &e); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Repo != "" {
		t.Errorf("Repo = %q, want empty string for empty array", e.Repo)
	}
}

func TestAuditLogEntry_UnmarshalJSON_NullRepo(t *testing.T) {
	data := `{"action":"org.update","repo":null}`
	var e AuditLogEntry
	if err := json.Unmarshal([]byte(data), &e); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Repo != "" {
		t.Errorf("Repo = %q, want empty string for null repo", e.Repo)
	}
}

func TestAuditLogEntry_UnmarshalJSON_MissingRepo(t *testing.T) {
	data := `{"action":"org.update"}`
	var e AuditLogEntry
	if err := json.Unmarshal([]byte(data), &e); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Repo != "" {
		t.Errorf("Repo = %q, want empty string when repo is absent", e.Repo)
	}
}

func TestAuditLogEntry_UnmarshalJSON_WithData(t *testing.T) {
	data := `{"action":"repo.create","repo":"myorg/repo1","data":{"key":"value","count":3}}`
	var e AuditLogEntry
	if err := json.Unmarshal([]byte(data), &e); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Data["key"] != "value" {
		t.Errorf("Data[key] = %v, want value", e.Data["key"])
	}
}

func TestAuditLogEntry_UnmarshalJSON_Malformed(t *testing.T) {
	data := `{"action": "repo.create", "repo": ` // truncated/invalid JSON
	var e AuditLogEntry
	if err := json.Unmarshal([]byte(data), &e); err == nil {
		t.Fatal("expected an error for malformed JSON, got nil")
	}
}

func TestAuditLogEntry_UnmarshalJSON_MalformedRepoArray(t *testing.T) {
	// repo looks like an array but contains invalid element types; the
	// implementation swallows this error and leaves Repo as its zero value
	// rather than failing the whole unmarshal.
	data := `{"action":"repo.access","repo":[123, 456]}`
	var e AuditLogEntry
	if err := json.Unmarshal([]byte(data), &e); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.Repo != "" {
		t.Errorf("Repo = %q, want empty string when array elements are not strings", e.Repo)
	}
}
