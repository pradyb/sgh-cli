package model

import (
	"strconv"
	"strings"
)

type Repositories struct {
	Repositories []Repository
}

type Repository struct {
	Id                    int    `json:"id"`
	Name                  string `json:"name"`
	Private               bool   `json:"private"`
	Description           string `json:"description"`
	DefaultBranch         string `json:"default_branch"`
	HTMLUrl               string `json:"html_url"`
	SSHUrl                string `json:"ssh_url"`
	CloneUrl              string `json:"clone_url"`
	IssuesUrl             string `json:"issues_url"`
	OpenIssuesCount       int    `json:"open_issues_count"`
	OpenPullRequestsCount int    `json:"open_pull_requests_count"`
	Language              string `json:"language"`
	Size                  int    `json:"size"`
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
	PRNumber         int      `json:"number"`
	TitleName        string   `json:"title"`
	Body             string   `json:"body"`
	State            string   `json:"state"`
	HTMLUrl          string   `json:"html_url"`
	Head             PRBranch `json:"head"`
	Base             PRBranch `json:"base"`
	Author           User     `json:"user"`
	Assignees        []User   `json:"assignees"`
	Reviewers        []Actor  `json:"requested_reviewers"`
	ReviewDecision   string   `json:"review_decision"`
	Merged           bool     `json:"merged"`
	IsMergeable      bool     `json:"mergeable"`
	Mergeable        string
	MergeStateStatus string `json:"mergeable_state"`
	MergedBy         User   `json:"merged_by"`
	MergeAt          string `json:"merged_at"`
	Comments         int    `json:"comments"`
	ReviewComments   int    `json:"review_comments"`
	Commits          int    `json:"commits"`
	ChangedFiles     int    `json:"changed_files"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	ErrorMessage     string
}

func (pr PullRequestResponse) RepositoryName() string {
	return pr.Base.Repo.Name
}

func (pr PullRequestResponse) AuthorName() string {
	if pr.Author.Name == "" {
		return pr.Author.Login
	}
	return pr.Author.Name
}

func (pr PullRequestResponse) AssigneesName() string {
	assignees := make([]string, 0)
	for _, assignee := range pr.Assignees {
		assignees = append(assignees, assignee.Name)
	}
	return strings.Join(assignees, "\n")
}

func (pr PullRequestResponse) ReviewersName() string {
	reviewers := make([]string, 0)
	for _, reviewer := range pr.Reviewers {
		reviewers = append(reviewers, reviewer.Name())
	}
	return strings.Join(reviewers, "\n")
}

func (pr PullRequestResponse) FirstReviewerName() string {
	if len(pr.Reviewers) == 0 {
		return ""
	}
	if len(pr.Reviewers) > 1 {
		return pr.Reviewers[0].Name() + "..."
	} else {
		return pr.Reviewers[0].Name()
	}
}

func (pr PullRequestResponse) Title() string {
	return strconv.Itoa(pr.PRNumber) + " " + pr.TitleName + " (" + pr.RepositoryName() + ")"
}

func (pr PullRequestResponse) Description() string {
	if pr.FirstReviewerName() == "" {
		return pr.AuthorName() + " " + pr.Base.Ref + " < " + pr.Head.Ref
	}
	return pr.AuthorName() + " " + pr.FirstReviewerName() + " " + pr.Base.Ref + " < " + pr.Head.Ref
}

func (pr PullRequestResponse) FilterValue() string {
	return pr.Title()
}

type Actor struct {
	Type string
	User User
	Team OrgTeam
}

func (a Actor) Name() string {
	if a.Type == "User" {
		return a.User.Name
	}
	return a.Team.Name
}

type User struct {
	Id         int    `json:"id"`
	Login      string `json:"login"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	Bio        string `json:"bio"`
	WebsiteUrl string `json:"websiteUrl"`
}
type PRBranch struct {
	Label string     `json:"label"`
	Ref   string     `json:"ref"`
	Sha   string     `json:"sha"`
	Repo  Repository `json:"repo"`
}

type PullRequestFilesResponse struct {
	RepositoryName string
	PRNumber       int
	Files          []PullRequestFile
	ErrorMessage   string
}

type PullRequestFile struct {
	Sha        string `json:"sha"`
	Filename   string `json:"filename"`
	Additions  int    `json:"additions"`
	Deletions  int    `json:"deletions"`
	Changes    int    `json:"changes"`
	ChangeType string `json:"status"`
}

type MergeResponse struct {
	RepositoryName string
	Merged         bool   `json:"merged"`
	Message        string `json:"message"`
	SHA            string `json:"sha"`
	ErrorMessage   string
}

type ReviewPullRequestResponse struct {
	RepositoryName string
	PRNumber       int
	Id             int    `json:"id"`
	User           User   `json:"user"`
	State          string `json:"state"`
	Body           string `json:"body"`
	CreatedAt      string `json:"created_at"`
	SubmittedAt    string `json:"submitted_at"`
	CommitId       string `json:"commit_id"`
	ErrorMessage   string
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
	Name                           string
	Type                           string
	LockBranch                     bool
	EnforceAdmins                  bool
	RequiredConversationResolution bool
	RequiredPullRequestReviews     RequiredPullRequestReviews `json:"required_pull_request_reviews"`
	RequiredStatusChecks           RequiredStatusChecks       `json:"required_status_checks"`
	Restrictions                   Restriction                `json:"restrictions"`
	RepositoryRulesetNames         []string
	ErrorMessage                   string
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

type RequiredPullRequestReviews struct {
	DismissStaleReviews            bool     `json:"dismiss_stale_reviews"`
	RequireCodeOwnerReviews        bool     `json:"require_code_owner_reviews"`
	RequiredApprovingReviewCount   int      `json:"required_approving_review_count"`
	RequireLastPushApproval        bool     `json:"require_last_push_approval"`
	BypassPullRequestAllowances    UserTeam `json:"bypass_pull_request_allowances"`
	RequiredReviewThreadResolution bool
}

type RequiredStatusChecks struct {
	Strict   bool     `json:"strict"`
	Contexts []string `json:"contexts"`
	Checks   []Check  `json:"checks"`
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
	Stats struct {
		Additions int `json:"additions"`
		Deletions int `json:"deletions"`
		Total     int `json:"total"`
	} `json:"stats"`
	Files []struct {
		Sha         string `json:"sha"`
		Filename    string `json:"filename"`
		Status      string `json:"status"`
		Additions   int    `json:"additions"`
		Deletions   int    `json:"deletions"`
		Changes     int    `json:"changes"`
		BlobUrl     string `json:"blob_url"`
		RawUrl      string `json:"raw_url"`
		ContentsUrl string `json:"contents_url"`
	} `json:"files"`

	ErrorMessage string
}

func (cr CommitResponse) RepoName() string {
	str := cr.HtmlUrl
	str1 := str[0:strings.Index(str, "/commit")]
	return str1[strings.LastIndex(str1, "/")+1:]
}

type CheckRunResponse struct {
	RepositoryName    string
	Total             int        `json:"total"`
	CheckRuns         []CheckRun `json:"check_runs"`
	OverallConclusion string
	ErrorMessage      string
}

type CheckRun struct {
	Name        string         `json:"name"`
	Status      string         `json:"status"`
	Conclusion  string         `json:"conclusion"`
	StartedAt   string         `json:"started_at"`
	CompletedAt string         `json:"completed_at"`
	DetailsUrl  string         `json:"details_url"`
	Output      CheckRunOutput `json:"output"`
}

type CheckRunOutput struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Text    string `json:"text"`
}

type OrgTeam struct {
	Name              string
	TotalMembers      int
	Url               string
	Members           []OrgTeamMember
	RepositoriesCount int
}

type OrgTeamMember struct {
	Login     string
	Name      string
	Url       string
	PeopleUrl string
}
