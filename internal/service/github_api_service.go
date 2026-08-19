// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/shurcooL/githubv4"

	"github.com/pradyb/sgh-cli/internal/model"
	appcontext "github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/logger"
)

var githubBaseURL = "https://api.github.com"

const (
	UPDATE_REF_URI       = "%s/repos/%s/%s/git/refs"
	PROTECTED_BRANCH_URI = "%s/repos/%s/%s/branches/%s/protection"
)

type apiResponse struct {
	Body       []byte
	LinkHeader string
}

func invokeAPI(ctx *appcontext.Context, method, url string, reqBody io.Reader) ([]byte, error) {
	resp, err := invokeAPIFull(context.Background(), ctx, method, url, reqBody)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func invokeAPIFull(reqCtx context.Context, ctx *appcontext.Context, method, url string, reqBody io.Reader) (*apiResponse, error) {
	req, err := http.NewRequestWithContext(reqCtx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	res, err := ctx.HttpClient.SendWithContext(reqCtx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return &apiResponse{Body: body, LinkHeader: res.Header.Get("Link")}, nil
}

func parseLinkNext(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}
	for _, part := range strings.Split(linkHeader, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, `rel="next"`) {
			start := strings.Index(part, "<")
			end := strings.Index(part, ">")
			if start >= 0 && end > start {
				return part[start+1 : end]
			}
		}
	}
	return ""
}

type refResponse struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

// GetOwnerType returns "Organization" or "User" for the given GitHub login.
func GetOwnerType(ctx *appcontext.Context, login string) (string, error) {
	body, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/users/%s", githubBaseURL, login), nil)
	if err != nil {
		return "", err
	}
	var result struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	return result.Type, nil
}

// UpdateRepoArchived sets the archived state of a repository (true = archive, false = unarchive).
func UpdateRepoArchived(ctx *appcontext.Context, orgName, repoName string, archived bool) error {
	payload := map[string]interface{}{"archived": archived}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = invokeAPI(ctx, "PATCH",
		fmt.Sprintf("%s/repos/%s/%s", githubBaseURL, orgName, repoName),
		bytes.NewReader(jsonBody),
	)
	return err
}

// UpdateRepoVisibility sets a repository to "public" or "private".
func UpdateRepoVisibility(ctx *appcontext.Context, orgName, repoName, visibility string) error {
	payload := map[string]interface{}{"visibility": visibility}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = invokeAPI(ctx, "PATCH",
		fmt.Sprintf("%s/repos/%s/%s", githubBaseURL, orgName, repoName),
		bytes.NewReader(jsonBody),
	)
	return err
}

func CreateNewBranch(ctx *appcontext.Context, orgName, repoName, newBranchName, refBranchName string) (model.RefResponse, error) {
	commitSHAResponse, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/git/ref/heads/%s", githubBaseURL, orgName, repoName, refBranchName), nil)
	if err != nil {
		return model.RefResponse{}, err
	}
	var refResponse refResponse
	if err := json.Unmarshal(commitSHAResponse, &refResponse); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the response body")
		return model.RefResponse{}, err
	}

	branchRequest := model.BranchRequest{Ref: "refs/heads/" + newBranchName, SHA: refResponse.Object.SHA}
	jsonBody, err := json.Marshal(branchRequest)
	if err != nil {
		return model.RefResponse{}, err
	}
	branchResponseByte, err := invokeAPI(ctx, "POST", fmt.Sprintf(UPDATE_REF_URI, githubBaseURL, orgName, repoName), bytes.NewReader(jsonBody))
	if err != nil {
		return model.RefResponse{}, err
	}
	var branchResponse model.RefResponse
	if err := json.Unmarshal(branchResponseByte, &branchResponse); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the new branch response body")
		return model.RefResponse{}, err
	}
	return branchResponse, nil
}

func CreateNewBranchFromCommit(ctx *appcontext.Context, orgName, repoName, newBranchName, commitSHA string) (model.RefResponse, error) {
	branchRequest := model.BranchRequest{Ref: "refs/heads/" + newBranchName, SHA: commitSHA}
	jsonBody, err := json.Marshal(branchRequest)
	if err != nil {
		return model.RefResponse{}, err
	}
	branchResponseByte, err := invokeAPI(ctx, "POST", fmt.Sprintf(UPDATE_REF_URI, githubBaseURL, orgName, repoName), bytes.NewReader(jsonBody))
	if err != nil {
		return model.RefResponse{}, err
	}
	var branchResponse model.RefResponse
	if err := json.Unmarshal(branchResponseByte, &branchResponse); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the branch response body")
		return model.RefResponse{}, err
	}
	return branchResponse, nil
}

func DeleteBranch(ctx *appcontext.Context, orgName, repoName, branchName string) (bool, error) {
	_, err := invokeAPI(ctx, "DELETE", fmt.Sprintf("%s/repos/%s/%s/git/refs/heads/%s", githubBaseURL, orgName, repoName, branchName), nil)
	if err != nil {
		return false, err
	}
	return true, nil
}

// RenameBranch renames a branch using the GitHub API.
func RenameBranch(ctx *appcontext.Context, orgName, repoName, oldName, newName string) error {
	payload := map[string]string{"new_name": newName}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = invokeAPI(ctx, "POST",
		fmt.Sprintf("%s/repos/%s/%s/branches/%s/rename", githubBaseURL, orgName, repoName, oldName),
		bytes.NewReader(jsonBody),
	)
	return err
}

func ListBranches(ctx *appcontext.Context, orgName, repoName string) ([]model.BranchResponse, error) {
	var allBranches []model.BranchResponse
	url := fmt.Sprintf("%s/repos/%s/%s/branches?per_page=100", githubBaseURL, orgName, repoName)

	for url != "" {
		resp, err := invokeAPIFull(context.Background(), ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		var page []model.BranchResponse
		if err := json.Unmarshal(resp.Body, &page); err != nil {
			logger.Flog.Error().Err(err).Msg("Error in unmarshal the branches response body")
			return nil, err
		}
		allBranches = append(allBranches, page...)
		url = parseLinkNext(resp.LinkHeader)
	}
	return allBranches, nil
}

type refTagResponse struct {
	SHA string `json:"sha"`
}

func CreateNewTag(ctx *appcontext.Context, orgName, repoName, tagName, refBranchName, message string) (model.RefResponse, error) {
	refCommitSHAResponse, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/git/ref/heads/%s", githubBaseURL, orgName, repoName, refBranchName), nil)
	if err != nil {
		return model.RefResponse{}, err
	}
	var refResponse refResponse
	if err := json.Unmarshal(refCommitSHAResponse, &refResponse); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the tag response body")
		return model.RefResponse{}, err
	}

	tagRequest := model.TagRequest{Tag: tagName, Object: refResponse.Object.SHA, Message: message, Type: "commit", Tagger: model.Tagger{Name: ctx.Config.TaggerName(orgName), Email: ctx.Config.TaggerEmail(orgName)}}
	jsonBody, err := json.Marshal(tagRequest)
	if err != nil {
		return model.RefResponse{}, err
	}
	tagCommitSHAResponse, err := invokeAPI(ctx, "POST", fmt.Sprintf("%s/repos/%s/%s/git/tags", githubBaseURL, orgName, repoName), bytes.NewReader(jsonBody))
	if err != nil {
		return model.RefResponse{}, err
	}
	var tagCommitResponse refTagResponse
	if err := json.Unmarshal(tagCommitSHAResponse, &tagCommitResponse); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the tag commit sha response body")
		return model.RefResponse{}, err
	}

	tagNewRequest := model.BranchRequest{Ref: "refs/tags/" + tagName, SHA: tagCommitResponse.SHA}
	tagByteRequest, err := json.Marshal(tagNewRequest)
	if err != nil {
		return model.RefResponse{}, err
	}
	tagByteResponse, err := invokeAPI(ctx, "POST", fmt.Sprintf(UPDATE_REF_URI, githubBaseURL, orgName, repoName), bytes.NewReader(tagByteRequest))
	if err != nil {
		return model.RefResponse{}, err
	}
	var tagResponse model.RefResponse
	if err := json.Unmarshal(tagByteResponse, &tagResponse); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the tag response body")
		return model.RefResponse{}, err
	}

	return tagResponse, nil
}

func ListTags(ctx *appcontext.Context, orgName, repoName string) ([]model.TagResponse, error) {
	var allTags []model.TagResponse
	url := fmt.Sprintf("%s/repos/%s/%s/tags?per_page=100", githubBaseURL, orgName, repoName)

	for url != "" {
		resp, err := invokeAPIFull(context.Background(), ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		var page []model.TagResponse
		if err := json.Unmarshal(resp.Body, &page); err != nil {
			logger.Flog.Error().Err(err).Msg("Error in unmarshal the tags response body")
			return nil, err
		}
		allTags = append(allTags, page...)
		url = parseLinkNext(resp.LinkHeader)
	}
	return allTags, nil
}

func DeleteTag(ctx *appcontext.Context, orgName, repoName, tagName string) (bool, error) {
	_, err := invokeAPI(ctx, "DELETE", fmt.Sprintf("%s/repos/%s/%s/git/refs/tags/%s", githubBaseURL, orgName, repoName, tagName), nil)
	if err != nil {
		return false, err
	}
	return true, nil
}

func CreateNewPullRequest(ctx *appcontext.Context, orgName, repoName, title, body, baseRef, headRef string) (model.PullRequestResponse, error) {
	prRequest := map[string]interface{}{
		"title": title,
		"body":  body,
		"head":  headRef,
		"base":  baseRef,
	}
	jsonBody, err := json.Marshal(prRequest)
	if err != nil {
		return model.PullRequestResponse{}, err
	}
	prResponseByte, err := invokeAPI(ctx, "POST", fmt.Sprintf("%s/repos/%s/%s/pulls", githubBaseURL, orgName, repoName), bytes.NewReader(jsonBody))
	if err != nil {
		return model.PullRequestResponse{}, err
	}
	var prResponse model.PullRequestResponse
	if err := json.Unmarshal(prResponseByte, &prResponse); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the new PR response body")
		return model.PullRequestResponse{}, err
	}
	return prResponse, nil
}

func GetPullRequestInfo(ctx *appcontext.Context, orgName, repoName string, prNumber int) (model.PullRequestResponse, error) {
	prResponseByte, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/pulls/%d", githubBaseURL, orgName, repoName, prNumber), nil)
	if err != nil {
		return model.PullRequestResponse{}, err
	}
	var prResponse model.PullRequestResponse
	if err := json.Unmarshal(prResponseByte, &prResponse); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the PR response body while getting the PR info")
		return model.PullRequestResponse{}, err
	}
	return prResponse, nil
}

func AddIssueAssignees(ctx *appcontext.Context, orgName, repoName string, prNumber int, assignees []string) (interface{}, error) {
	assigneesRequest := map[string]interface{}{
		"assignees": assignees,
	}
	jsonBody, err := json.Marshal(assigneesRequest)
	if err != nil {
		return nil, err
	}
	_, err = invokeAPI(ctx, "POST", fmt.Sprintf("%s/repos/%s/%s/issues/%d/assignees", githubBaseURL, orgName, repoName, prNumber), bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	return fmt.Sprintf("Assignees %s added successfully", assignees), nil
}

func AddReviewers(ctx *appcontext.Context, orgName, repoName string, prNumber int, reviewers []string) (interface{}, error) {
	reviewersRequest := map[string]interface{}{
		"reviewers": reviewers,
	}
	jsonBody, err := json.Marshal(reviewersRequest)
	if err != nil {
		return nil, err
	}
	_, err = invokeAPI(ctx, "POST", fmt.Sprintf("%s/repos/%s/%s/pulls/%d/requested_reviewers", githubBaseURL, orgName, repoName, prNumber), bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	return fmt.Sprintf("Reviewers %s added successfully", reviewers), nil
}

func ListPullRequests(ctx *appcontext.Context, orgName, repoName string, baseRef, headRef string, all bool) ([]model.PullRequestResponse, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/pulls?per_page=100", githubBaseURL, orgName, repoName)
	if baseRef != "" {
		apiURL = fmt.Sprintf("%s&base=%s", apiURL, baseRef)
	}
	if headRef != "" {
		apiURL = fmt.Sprintf("%s&head=%s:%s", apiURL, orgName, headRef)
	}
	if all {
		apiURL = fmt.Sprintf("%s&state=all", apiURL)
	}

	var allPRs []model.PullRequestResponse
	for apiURL != "" {
		resp, err := invokeAPIFull(context.Background(), ctx, "GET", apiURL, nil)
		if err != nil {
			return nil, err
		}
		var page []model.PullRequestResponse
		if err := json.Unmarshal(resp.Body, &page); err != nil {
			logger.Flog.Error().Err(err).Msg("Error in unmarshal the PR response body")
			return nil, err
		}
		allPRs = append(allPRs, page...)
		apiURL = parseLinkNext(resp.LinkHeader)
	}
	return allPRs, nil
}

func UpdatePullRequest(ctx *appcontext.Context, orgName, repoName string, prNumber int, state string) (model.PullRequestResponse, error) {
	prRequest := map[string]interface{}{
		"state": state,
	}
	jsonBody, err := json.Marshal(prRequest)
	if err != nil {
		return model.PullRequestResponse{}, err
	}
	prResponseByte, err := invokeAPI(ctx, "PATCH", fmt.Sprintf("%s/repos/%s/%s/pulls/%d", githubBaseURL, orgName, repoName, prNumber), bytes.NewReader(jsonBody))
	if err != nil {
		return model.PullRequestResponse{}, err
	}
	var prResponse model.PullRequestResponse
	if err := json.Unmarshal(prResponseByte, &prResponse); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the PR response body")
		return model.PullRequestResponse{}, err
	}
	return prResponse, nil
}

func ListPullRequestReviews(ctx *appcontext.Context, orgName, repoName string, prNumber int) ([]model.ReviewPullRequestResponse, error) {
	reviewResponseByte, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews", githubBaseURL, orgName, repoName, prNumber), nil)
	if err != nil {
		return nil, err
	}
	var reviewResponses []model.ReviewPullRequestResponse
	if err := json.Unmarshal(reviewResponseByte, &reviewResponses); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the Review PR response body")
		return nil, err
	}
	return reviewResponses, nil
}

func GetPullRequestFiles(ctx *appcontext.Context, orgName, repoName string, prNumber int) ([]model.PullRequestFile, error) {
	filesResponseByte, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/pulls/%d/files", githubBaseURL, orgName, repoName, prNumber), nil)
	if err != nil {
		return nil, err
	}
	var files []model.PullRequestFile
	if err := json.Unmarshal(filesResponseByte, &files); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the PR files response body")
		return nil, err
	}
	return files, nil
}

func ReviewPullRequest(ctx *appcontext.Context, orgName, repoName string, prNumber int, action, body string) (model.ReviewPullRequestResponse, error) {
	reviewRequest := map[string]interface{}{
		"event": strings.ToUpper(action),
		"body":  body,
	}
	jsonBody, err := json.Marshal(reviewRequest)
	if err != nil {
		return model.ReviewPullRequestResponse{RepositoryName: repoName, PRNumber: prNumber}, err
	}
	reviewResponseByte, err := invokeAPI(ctx, "POST", fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews", githubBaseURL, orgName, repoName, prNumber), bytes.NewReader(jsonBody))
	if err != nil {
		return model.ReviewPullRequestResponse{RepositoryName: repoName, PRNumber: prNumber}, err
	}
	logger.Flog.Info().Str("org", orgName).Str("repo", repoName).Int("pr", prNumber).Str("event", action).Msgf("PR Review successfully")
	var reviewResponse model.ReviewPullRequestResponse
	if err := json.Unmarshal(reviewResponseByte, &reviewResponse); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the Review PR response body")
		return model.ReviewPullRequestResponse{RepositoryName: repoName, PRNumber: prNumber}, err
	}
	reviewResponse.RepositoryName = repoName
	reviewResponse.PRNumber = prNumber
	return reviewResponse, nil
}

func MergePullRequest(ctx *appcontext.Context, orgName, repoName string, prNumber int, title, body string) (model.MergeResponse, error) {
	mergeRequest := map[string]interface{}{}
	if title != "" {
		mergeRequest["commit_title"] = title
	}
	if body != "" {
		mergeRequest["commit_message"] = body
	}
	jsonBody, err := json.Marshal(mergeRequest)
	if err != nil {
		return model.MergeResponse{}, err
	}
	mergeResponseByte, err := invokeAPI(ctx, "PUT", fmt.Sprintf("%s/repos/%s/%s/pulls/%d/merge", githubBaseURL, orgName, repoName, prNumber), bytes.NewReader(jsonBody))
	if err != nil {
		return model.MergeResponse{}, err
	}
	logger.Flog.Info().Str("org", orgName).Str("repo", repoName).Int("pr", prNumber).Msgf("PR Merged successfully")
	var mergeResponse model.MergeResponse
	if err := json.Unmarshal(mergeResponseByte, &mergeResponse); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the Merge response body")
		return model.MergeResponse{}, err
	}
	return mergeResponse, nil
}

func UpdateProtectedBranch(ctx *appcontext.Context, orgName, repoName, branchName string, payload []byte) (model.ProtectedBranch, error) {
	response, err := invokeAPI(ctx, "PUT", fmt.Sprintf(PROTECTED_BRANCH_URI, githubBaseURL, orgName, repoName, branchName), bytes.NewReader(payload))
	if err != nil {
		return model.ProtectedBranch{RepositoryName: repoName}, err
	}
	var branch model.ProtectedBranch
	if err := json.Unmarshal(response, &branch); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the branch response body")
		return model.ProtectedBranch{RepositoryName: repoName}, err
	}
	branch.RepositoryName = repoName
	branch.Type = "Branch Protection"
	return branch, nil
}

func DeleteProtectedBranch(ctx *appcontext.Context, orgName, repoName, branchName string) (bool, error) {
	_, err := invokeAPI(ctx, "DELETE", fmt.Sprintf(PROTECTED_BRANCH_URI, githubBaseURL, orgName, repoName, branchName), nil)
	if err != nil {
		return false, err
	}
	return true, nil
}

func ListCommits(ctx *appcontext.Context, orgName, repoName, branchName, since, until string) ([]model.CommitResponse, error) {
	var allCommits []model.CommitResponse
	url := fmt.Sprintf("%s/repos/%s/%s/commits?per_page=100&since=%s&sha=%s", githubBaseURL, orgName, repoName, since, branchName)
	if until != "" {
		url += "&until=" + until
	}

	for url != "" {
		resp, err := invokeAPIFull(context.Background(), ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		var page []model.CommitResponse
		if err := json.Unmarshal(resp.Body, &page); err != nil {
			logger.Flog.Error().Err(err).Msg("Error in unmarshal the commits response body")
			return nil, err
		}
		allCommits = append(allCommits, page...)
		url = parseLinkNext(resp.LinkHeader)
	}
	return allCommits, nil
}

func GetCommitInfo(ctx *appcontext.Context, orgName, repoName, commitSha string) (model.CommitResponse, error) {
	response, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/commits/%s", githubBaseURL, orgName, repoName, commitSha), nil)
	if err != nil {
		return model.CommitResponse{}, err
	}
	var commit model.CommitResponse
	if err := json.Unmarshal(response, &commit); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the commit response body")
		return model.CommitResponse{}, err
	}
	return commit, nil
}

func GetCommitCheckRuns(ctx *appcontext.Context, orgName, repoName, commitSha string) (model.CheckRunResponse, error) {
	response, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/commits/%s/check-runs", githubBaseURL, orgName, repoName, commitSha), nil)
	if err != nil {
		return model.CheckRunResponse{}, err
	}
	var checkRuns model.CheckRunResponse
	if err := json.Unmarshal(response, &checkRuns); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the check runs response body")
		return model.CheckRunResponse{}, err
	}
	return checkRuns, nil
}

func ListWorkflowRuns(ctx *appcontext.Context, orgName, repoName, branch, status string, count int) ([]model.WorkflowRun, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs?per_page=%d", githubBaseURL, orgName, repoName, count)
	if branch != "" {
		url = fmt.Sprintf("%s&branch=%s", url, branch)
	}
	if status != "" {
		url = fmt.Sprintf("%s&status=%s", url, status)
	}
	response, err := invokeAPI(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	var runsResponse model.WorkflowRunsResponse
	if err := json.Unmarshal(response, &runsResponse); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the workflow runs response body")
		return nil, err
	}
	return runsResponse.WorkflowRuns, nil
}

// DispatchWorkflow triggers a workflow_dispatch event for a named workflow.
// inputs is a map of input key→value pairs (may be nil).
func DispatchWorkflow(ctx *appcontext.Context, orgName, repoName, workflowID, ref string, inputs map[string]string) error {
	payload := map[string]interface{}{
		"ref": ref,
	}
	if len(inputs) > 0 {
		payload["inputs"] = inputs
	}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = invokeAPI(ctx, "POST",
		fmt.Sprintf("%s/repos/%s/%s/actions/workflows/%s/dispatches", githubBaseURL, orgName, repoName, workflowID),
		bytes.NewReader(jsonBody),
	)
	return err
}

func RerunWorkflowRun(ctx *appcontext.Context, orgName, repoName string, runID int) (bool, error) {
	_, err := invokeAPI(ctx, "POST", fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d/rerun", githubBaseURL, orgName, repoName, runID), nil)
	if err != nil {
		return false, err
	}
	return true, nil
}

func CancelWorkflowRun(ctx *appcontext.Context, orgName, repoName string, runID int) (bool, error) {
	_, err := invokeAPI(ctx, "POST", fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d/cancel", githubBaseURL, orgName, repoName, runID), nil)
	if err != nil {
		return false, err
	}
	return true, nil
}

func GetWorkflowRun(ctx *appcontext.Context, orgName, repoName string, runID int) (model.WorkflowRun, error) {
	response, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d", githubBaseURL, orgName, repoName, runID), nil)
	if err != nil {
		return model.WorkflowRun{}, err
	}
	var run model.WorkflowRun
	if err := json.Unmarshal(response, &run); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the workflow run response body")
		return model.WorkflowRun{}, err
	}
	return run, nil
}

func GetWorkflowRunJobs(ctx *appcontext.Context, orgName, repoName string, runID int) ([]model.WorkflowJob, error) {
	response, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d/jobs", githubBaseURL, orgName, repoName, runID), nil)
	if err != nil {
		return nil, err
	}
	var jobsResponse model.WorkflowJobsResponse
	if err := json.Unmarshal(response, &jobsResponse); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the workflow jobs response body")
		return nil, err
	}
	return jobsResponse.Jobs, nil
}

func ListSecretScanningAlerts(ctx *appcontext.Context, orgName, repoName, state string) ([]model.SecretScanningAlert, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/secret-scanning/alerts?per_page=100", githubBaseURL, orgName, repoName)
	if state != "" {
		url = fmt.Sprintf("%s&state=%s", url, state)
	}

	var allAlerts []model.SecretScanningAlert
	for url != "" {
		resp, err := invokeAPIFull(context.Background(), ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		var page []model.SecretScanningAlert
		if err := json.Unmarshal(resp.Body, &page); err != nil {
			logger.Flog.Error().Err(err).Msg("Error in unmarshal the secret scanning alerts response body")
			return nil, err
		}
		for i := range page {
			page[i].RepositoryName = repoName
		}
		allAlerts = append(allAlerts, page...)
		url = parseLinkNext(resp.LinkHeader)
	}
	return allAlerts, nil
}

func GetSecretScanningAlert(ctx *appcontext.Context, orgName, repoName string, alertNumber int) (model.SecretScanningAlert, error) {
	response, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/secret-scanning/alerts/%d", githubBaseURL, orgName, repoName, alertNumber), nil)
	if err != nil {
		return model.SecretScanningAlert{}, err
	}
	var alert model.SecretScanningAlert
	if err := json.Unmarshal(response, &alert); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the secret scanning alert response body")
		return model.SecretScanningAlert{}, err
	}
	alert.RepositoryName = repoName
	return alert, nil
}

func UpdateSecretScanningAlert(ctx *appcontext.Context, orgName, repoName string, alertNumber int, state, resolution, resolutionComment string) (model.SecretScanningAlert, error) {
	updatePayload := map[string]interface{}{
		"state": state,
	}

	if state == "resolved" && resolution != "" {
		updatePayload["resolution"] = resolution
		if resolutionComment != "" {
			updatePayload["resolution_comment"] = resolutionComment
		}
	}

	payloadBytes, err := json.Marshal(updatePayload)
	if err != nil {
		return model.SecretScanningAlert{}, fmt.Errorf("failed to marshal update payload: %w", err)
	}

	response, err := invokeAPI(ctx, "PATCH", fmt.Sprintf("%s/repos/%s/%s/secret-scanning/alerts/%d", githubBaseURL, orgName, repoName, alertNumber), bytes.NewBuffer(payloadBytes))
	if err != nil {
		return model.SecretScanningAlert{}, err
	}

	var alert model.SecretScanningAlert
	if err := json.Unmarshal(response, &alert); err != nil {
		logger.Flog.Error().Err(err).Msg("Error in unmarshal the secret scanning alert response body")
		return model.SecretScanningAlert{}, err
	}
	alert.RepositoryName = repoName
	return alert, nil
}

func ListIssues(ctx *appcontext.Context, orgName, repoName, state, labels, assignee, creator string, perPage int) ([]model.IssueResponse, error) {
	if perPage <= 0 {
		perPage = 100
	}
	url := fmt.Sprintf("%s/repos/%s/%s/issues?per_page=%d&sort=created&direction=desc", githubBaseURL, orgName, repoName, perPage)
	if state != "" {
		url = fmt.Sprintf("%s&state=%s", url, state)
	}
	if labels != "" {
		url = fmt.Sprintf("%s&labels=%s", url, labels)
	}
	if assignee != "" {
		url = fmt.Sprintf("%s&assignee=%s", url, assignee)
	}
	if creator != "" {
		url = fmt.Sprintf("%s&creator=%s", url, creator)
	}

	var allIssues []model.IssueResponse
	for url != "" {
		resp, err := invokeAPIFull(context.Background(), ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		var page []model.IssueResponse
		if err := json.Unmarshal(resp.Body, &page); err != nil {
			logger.Flog.Error().Err(err).Msg("Error unmarshalling issues response")
			return nil, err
		}
		for i := range page {
			page[i].RepositoryName = repoName
		}
		allIssues = append(allIssues, page...)
		url = parseLinkNext(resp.LinkHeader)
	}
	return allIssues, nil
}

func GetIssue(ctx *appcontext.Context, orgName, repoName string, issueNumber int) (model.IssueResponse, error) {
	response, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/issues/%d", githubBaseURL, orgName, repoName, issueNumber), nil)
	if err != nil {
		return model.IssueResponse{}, err
	}
	var issue model.IssueResponse
	if err := json.Unmarshal(response, &issue); err != nil {
		logger.Flog.Error().Err(err).Msg("Error unmarshalling issue response")
		return model.IssueResponse{}, err
	}
	issue.RepositoryName = repoName
	return issue, nil
}

// CreateIssue creates a new issue in a repository.
func CreateIssue(ctx *appcontext.Context, orgName, repoName, title, body, assignee string, labels []string) (model.IssueResponse, error) {
	payload := map[string]interface{}{
		"title": title,
	}
	if body != "" {
		payload["body"] = body
	}
	if assignee != "" {
		payload["assignees"] = []string{assignee}
	}
	if len(labels) > 0 {
		payload["labels"] = labels
	}
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return model.IssueResponse{}, err
	}
	response, err := invokeAPI(ctx, "POST",
		fmt.Sprintf("%s/repos/%s/%s/issues", githubBaseURL, orgName, repoName),
		bytes.NewReader(jsonBody),
	)
	if err != nil {
		return model.IssueResponse{}, err
	}
	var issue model.IssueResponse
	if err := json.Unmarshal(response, &issue); err != nil {
		logger.Flog.Error().Err(err).Msg("Error unmarshalling create issue response")
		return model.IssueResponse{}, err
	}
	issue.RepositoryName = repoName
	return issue, nil
}

// UpdateIssue patches an issue state ("open" or "closed").
func UpdateIssue(ctx *appcontext.Context, orgName, repoName string, issueNumber int, state string) error {
	body := fmt.Sprintf(`{"state":%q}`, state)
	_, err := invokeAPI(ctx, "PATCH",
		fmt.Sprintf("%s/repos/%s/%s/issues/%d", githubBaseURL, orgName, repoName, issueNumber),
		bytes.NewBufferString(body),
	)
	return err
}

func ListIssueComments(ctx *appcontext.Context, orgName, repoName string, issueNumber int) ([]model.IssueComment, error) {
	response, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/issues/%d/comments?per_page=100", githubBaseURL, orgName, repoName, issueNumber), nil)
	if err != nil {
		return nil, err
	}
	var comments []model.IssueComment
	if err := json.Unmarshal(response, &comments); err != nil {
		logger.Flog.Error().Err(err).Msg("Error unmarshalling issue comments response")
		return nil, err
	}
	return comments, nil
}

// GetAuditLog fetches the organization audit log using the GitHub REST API.
// phrase, include, order, after and before are optional filters; pass "" to omit.
func GetAuditLog(ctx *appcontext.Context, orgName string, phrase, include string, perPage int) ([]model.AuditLogEntry, error) {
	if perPage <= 0 {
		perPage = 100
	}
	url := fmt.Sprintf("%s/orgs/%s/audit-log?per_page=%d", githubBaseURL, orgName, perPage)
	if phrase != "" {
		url += "&phrase=" + phrase
	}
	if include != "" {
		url += "&include=" + include
	}
	response, err := invokeAPI(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	var entries []model.AuditLogEntry
	if err := json.Unmarshal(response, &entries); err != nil {
		logger.Flog.Error().Err(err).Msg("Error unmarshalling audit log response")
		return nil, err
	}
	return entries, nil
}

// orgNode is the per-org fragment used inside the viewer organizations query.
type orgNode struct {
	Login                           githubv4.String
	Name                            githubv4.String
	Description                     githubv4.String
	Email                           githubv4.String
	WebsiteURL                      githubv4.String `graphql:"websiteUrl"`
	Location                        githubv4.String
	TwitterUsername                 githubv4.String `graphql:"twitterUsername"`
	CreatedAt                       githubv4.DateTime
	UpdatedAt                       githubv4.DateTime
	URL                             githubv4.String `graphql:"url"`
	AvatarURL                       githubv4.String `graphql:"avatarUrl"`
	IsVerified                      githubv4.Boolean
	RequiresTwoFactorAuthentication githubv4.Boolean `graphql:"requiresTwoFactorAuthentication"`
	MembersWithRole                 struct {
		TotalCount githubv4.Int
	} `graphql:"membersWithRole"`
	Teams struct {
		TotalCount githubv4.Int
	} `graphql:"teams"`
	Repositories struct {
		TotalCount     githubv4.Int
		TotalDiskUsage githubv4.Int `graphql:"totalDiskUsage"`
	} `graphql:"repositories(privacy: PRIVATE, first: 1)"`
	PublicRepositories struct {
		TotalCount githubv4.Int
	} `graphql:"publicRepositories: repositories(privacy: PUBLIC, first: 1)"`
}

// viewerOrgsQuery pages through all organizations the authenticated user belongs to.
type viewerOrgsQuery struct {
	Viewer struct {
		Organizations struct {
			PageInfo model.PageInfo
			Nodes    []orgNode
		} `graphql:"organizations(first: 100, after: $cursor)"`
	}
}

func orgNodeToDetail(o orgNode) model.OrgDetail {
	totalRepos := int(o.Repositories.TotalCount) + int(o.PublicRepositories.TotalCount)
	return model.OrgDetail{
		Login:             string(o.Login),
		Name:              string(o.Name),
		Description:       string(o.Description),
		Email:             string(o.Email),
		WebsiteURL:        string(o.WebsiteURL),
		Location:          string(o.Location),
		TwitterUsername:   string(o.TwitterUsername),
		CreatedAt:         o.CreatedAt.Format("2006-01-02"),
		UpdatedAt:         o.UpdatedAt.Format("2006-01-02"),
		URL:               string(o.URL),
		AvatarURL:         string(o.AvatarURL),
		IsVerified:        bool(o.IsVerified),
		RequiresTwoFA:     bool(o.RequiresTwoFactorAuthentication),
		MembersCount:      int(o.MembersWithRole.TotalCount),
		TeamsCount:        int(o.Teams.TotalCount),
		ReposCount:        totalRepos,
		PublicReposCount:  int(o.PublicRepositories.TotalCount),
		PrivateReposCount: int(o.Repositories.TotalCount),
		DiskUsageMB:       float64(o.Repositories.TotalDiskUsage) / 1024.0,
	}
}

// ListOrganizations returns all organizations the authenticated token belongs to,
// using the viewer.organizations GraphQL query (paginated).
func ListOrganizations(ctx *appcontext.Context) ([]model.OrgDetail, error) {
	variables := map[string]interface{}{
		"cursor": (*githubv4.String)(nil),
	}
	var results []model.OrgDetail
	for {
		var q viewerOrgsQuery
		if err := QueryWithContext(context.Background(), ctx, &q, variables); err != nil {
			return nil, fmt.Errorf("graphql query failed for viewer organizations: %w", err)
		}
		for _, node := range q.Viewer.Organizations.Nodes {
			results = append(results, orgNodeToDetail(node))
		}
		if !q.Viewer.Organizations.PageInfo.HasNextPage {
			break
		}
		cursor := githubv4.String(q.Viewer.Organizations.PageInfo.EndCursor)
		variables["cursor"] = &cursor
	}
	return results, nil
}

// GetCurrentUser fetches the authenticated user's profile via GET /user.
func GetCurrentUser(ctx *appcontext.Context) (*model.UserInfo, error) {
	url := fmt.Sprintf("%s/user", githubBaseURL)
	body, err := invokeAPI(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %w", err)
	}
	var user model.UserInfo
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}
	return &user, nil
}
