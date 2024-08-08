package model

import (
	"strings"
)

type Repositories struct {
	Repositories []Repository
}

type Repository struct {
	Id              int    `json:"id"`
	Name            string `json:"name"`
	Private         bool   `json:"private"`
	Description     string `json:"description"`
	HTMLUrl         string `json:"html_url"`
	SSHUrl          string `json:"ssh_url"`
	CloneUrl        string `json:"clone_url"`
	OpenIssuesCount int    `json:"open_issues_count"`
	Language        string `json:"language"`
	Size            int    `json:"size"`
}

type BranchRequest struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

type TagRequest struct {
	Tag     string `json:"tag"`
	Object  string `json:"object"`
	Message string `json:"message"`
	Type    string `json:"type"`
	Tagger  Tagger `json:"tagger"`
}

type Tagger struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type PullRequestResponse struct {
	PRNumber     int      `json:"number"`
	Title        string   `json:"title"`
	Status       string   `json:"state"`
	HTMLUrl      string   `json:"html_url"`
	Head         PRBranch `json:"head"`
	Base         PRBranch `json:"base"`
	User         User     `json:"user"`
	Assignees    []User   `json:"assignees"`
	Reviewers    []User   `json:"requested_reviewers"`
	ErrorMessage string
}

func (pr PullRequestResponse) RepositoryName() string {
	return pr.Base.Repo.Name
}
func (pr PullRequestResponse) UserName() string {
	return pr.User.Login
}
func (pr PullRequestResponse) AssigneesName() string {
	assignees := make([]string, 0)
	for _, assignee := range pr.Assignees {
		assignees = append(assignees, assignee.Login)
	}
	return strings.Join(assignees, "\n")
}
func (pr PullRequestResponse) ReviewersName() string {
	reviewers := make([]string, 0)
	for _, reviewer := range pr.Reviewers {
		reviewers = append(reviewers, reviewer.Login)
	}
	return strings.Join(reviewers, "\n")
}

type User struct {
	Id    int    `json:"id"`
	Login string `json:"login"`
	Type  string `json:"type"`
}

type PRBranch struct {
	Label string     `json:"label"`
	Ref   string     `json:"ref"`
	Sha   string     `json:"sha"`
	Repo  Repository `json:"repo"`
}

type MergeResponse struct {
	Merged       bool   `json:"merged"`
	Message      string `json:"message"`
	SHA          string `json:"sha"`
	ErrorMessage string
}

type RefResponse struct {
	Ref    string `json:"ref"`
	NodeID string `json:"node_id"`
	Url    string `json:"url"`
	Object struct {
		SHA  string `json:"sha"`
		Type string `json:"type"`
		Url  string `json:"url"`
	} `json:"object"`
}

type RefUIResponse struct {
	RepositoryName string
	Ref            string
	Type           string
	SuccessMessage string
	ErrorMessage   string
}

func (c RefUIResponse) IsSuccess() bool {
	return c.ErrorMessage == ""
}

func CreateNewCommonResponse(repoName, ref, refType, successMessage, errorMessage string) RefUIResponse {
	return RefUIResponse{
		RepositoryName: repoName,
		Ref:            ref,
		Type:           refType,
		SuccessMessage: successMessage,
		ErrorMessage:   errorMessage,
	}
}

type ProtectedBranch struct {
	RepositoryName                 string
	LockBranch                     BoolData `json:"lock_branch"`
	EnforceAdmins                  BoolData `json:"enforce_admins"`
	RequiredConversationResolution BoolData `json:"required_conversation_resolution"`
	RequiredPullRequestReviews     struct {
		DismissStaleReviews          bool     `json:"dismiss_stale_reviews"`
		RequireCodeOwnerReviews      bool     `json:"require_code_owner_reviews"`
		RequiredApprovingReviewCount int      `json:"required_approving_review_count"`
		RequireLastPushApproval      bool     `json:"require_last_push_approval"`
		BypassPullRequestAllowances  UserTeam `json:"bypass_pull_request_allowances"`
	} `json:"required_pull_request_reviews"`
	RequiredStatusChecks struct {
		Strict   bool     `json:"strict"`
		Contexts []string `json:"contexts"`
		Checks   []Check  `json:"checks"`
	} `json:"required_status_checks"`
	Restrictions Restriction `json:"restrictions"`
	ErrorMessage string
}

type UserTeam struct {
	Users []User `json:"users"`
}

type Restriction struct {
	Users []User `json:"users"`
}

type Check struct {
	Context string `json:"context"`
	AppId   int    `json:"app_id"`
}

type BoolData struct {
	Enabled bool `json:"enabled"`
}

type ProtectedBranchRequest struct {
	RequiredStatusChecks struct {
		Strict bool           `json:"strict"`
		Checks []CheckRequest `json:"checks"`
	} `json:"required_status_checks"`
	RequiredPullRequestReviews struct {
		DismissStaleReviews          bool `json:"dismiss_stale_reviews"`
		RequireCodeOwnerReviews      bool `json:"require_code_owner_reviews"`
		RequiredApprovingReviewCount int  `json:"required_approving_review_count"`
		RequireLastPushApproval      bool `json:"require_last_push_approval"`
		BypassPullRequestAllowances  struct {
			Users []string `json:"users"`
			Teams []string `json:"teams"`
		} `json:"bypass_pull_request_allowances"`
	} `json:"required_pull_request_reviews"`
	RequiredSignatures             bool `json:"required_signatures"`
	EnforceAdmins                  bool `json:"enforce_admins"`
	RequiredLinearHistory          bool `json:"required_linear_history"`
	AllowForcePushes               bool `json:"allow_force_pushes"`
	AllowDeletions                 bool `json:"allow_deletions"`
	RequiredConversationResolution bool `json:"required_conversation_resolution"`
	LockBranch                     bool `json:"lock_branch"`
	AllowForkSyncing               bool `json:"allow_fork_syncing"`
	Restrictions                   struct {
		Users []string `json:"users"`
		Teams []string `json:"teams"`
		Apps  []string `json:"apps"`
	} `json:"restrictions"`
	BlockCreations bool `json:"block_creations"`
}

type CheckRequest struct {
	Context string `json:"context"`
	AppID   int    `json:"app_id"`
}

type PostReleaseResponse struct {
	RepositoryName string
	PRNumber       int
	PRHtmlUrl      string
	TagHtmlUrl     string
	TagCommitSHA   string
	ErrorMessage   string
}

type CommitResponse struct {
	RepositoryName string
	NodeID         string `json:"node_id"`
	Sha            string `json:"sha"`
	HtmlUrl        string `json:"html_url"`
	CommitUrl      string `json:"commit_url"`
	Author         User   `json:"author"`
	Committer      User   `json:"committer"`
	Commit         struct {
		Author struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Date  string `json:"date"`
		} `json:"author"`
		Committer struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Date  string `json:"date"`
		} `json:"committer"`
		Message string `json:"message"`
		Tree    struct {
			Sha string `json:"sha"`
			Url string `json:"url"`
		} `json:"tree"`
		CommentCount int `json:"comment_count"`
		Verification struct {
			Verified  bool   `json:"verified"`
			Reason    string `json:"reason"`
			Signature string `json:"signature"`
			Payload   string `json:"payload"`
		} `json:"verification"`
	} `json:"commit"`
	ErrorMessage string
}

func (cr CommitResponse) RepoName() string {
	str := cr.HtmlUrl
	str1 := str[0:strings.Index(str, "/commit")]
	return str1[strings.LastIndex(str1, "/")+1:]
}
