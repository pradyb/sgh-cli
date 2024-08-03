package model

import "strings"

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
	return strings.Join(assignees, ",")
}
func (pr PullRequestResponse) ReviewersName() string {
	reviewers := make([]string, 0)
	for _, reviewer := range pr.Reviewers {
		reviewers = append(reviewers, reviewer.Login)
	}
	return strings.Join(reviewers, ",")
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
