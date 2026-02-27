package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/pkg/apperrors"
	appcontext "github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
)

var GITHUB_BASE_URL = "https://api.github.com"

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

func invokeAPIWithContext(reqCtx context.Context, ctx *appcontext.Context, method, url string, reqBody io.Reader) ([]byte, error) {
	resp, err := invokeAPIFull(reqCtx, ctx, method, url, reqBody)
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

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, &apperrors.GitHubError{
			StatusCode: res.StatusCode,
			Message:    string(body),
			URL:        url,
		}
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

func CreateNewBranch(ctx *appcontext.Context, orgName, repoName, newBranchName, refBranchName string) (model.RefResponse, error) {
	commitSHAResponse, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/git/ref/heads/%s", GITHUB_BASE_URL, orgName, repoName, refBranchName), nil)
	if err != nil {
		return model.RefResponse{}, err
	}
	var refResponse refResponse
	if err := json.Unmarshal(commitSHAResponse, &refResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the response body")
		return model.RefResponse{}, err
	}

	branchRequest := model.BranchRequest{Ref: "refs/heads/" + newBranchName, SHA: refResponse.Object.SHA}
	jsonBody, err := json.Marshal(branchRequest)
	if err != nil {
		return model.RefResponse{}, err
	}
	branchResponseByte, err := invokeAPI(ctx, "POST", fmt.Sprintf(UPDATE_REF_URI, GITHUB_BASE_URL, orgName, repoName), bytes.NewReader(jsonBody))
	if err != nil {
		return model.RefResponse{}, err
	}
	var branchResponse model.RefResponse
	if err := json.Unmarshal(branchResponseByte, &branchResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the new branch response body")
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
	branchResponseByte, err := invokeAPI(ctx, "POST", fmt.Sprintf(UPDATE_REF_URI, GITHUB_BASE_URL, orgName, repoName), bytes.NewReader(jsonBody))
	if err != nil {
		return model.RefResponse{}, err
	}
	var branchResponse model.RefResponse
	if err := json.Unmarshal(branchResponseByte, &branchResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the branch response body")
		return model.RefResponse{}, err
	}
	return branchResponse, nil
}

func DeleteBranch(ctx *appcontext.Context, orgName, repoName, branchName string) (bool, error) {
	_, err := invokeAPI(ctx, "DELETE", fmt.Sprintf("%s/repos/%s/%s/git/refs/heads/%s", GITHUB_BASE_URL, orgName, repoName, branchName), nil)
	if err != nil {
		return false, err
	}
	return true, nil
}

func ListBranches(ctx *appcontext.Context, orgName, repoName string) ([]model.BranchResponse, error) {
	var allBranches []model.BranchResponse
	url := fmt.Sprintf("%s/repos/%s/%s/branches?per_page=100", GITHUB_BASE_URL, orgName, repoName)

	for url != "" {
		resp, err := invokeAPIFull(context.Background(), ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		var page []model.BranchResponse
		if err := json.Unmarshal(resp.Body, &page); err != nil {
			logger.Glog.Error().Err(err).Msg("Error in unmarshal the branches response body")
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
	refCommitSHAResponse, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/git/ref/heads/%s", GITHUB_BASE_URL, orgName, repoName, refBranchName), nil)
	if err != nil {
		return model.RefResponse{}, err
	}
	var refResponse refResponse
	if err := json.Unmarshal(refCommitSHAResponse, &refResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the tag response body")
		return model.RefResponse{}, err
	}

	tagRequest := model.TagRequest{Tag: tagName, Object: refResponse.Object.SHA, Message: message, Type: "commit", Tagger: model.Tagger{Name: ctx.Config.TaggerName(orgName), Email: ctx.Config.TaggerEmail(orgName)}}
	jsonBody, err := json.Marshal(tagRequest)
	if err != nil {
		return model.RefResponse{}, err
	}
	tagCommitSHAResponse, err := invokeAPI(ctx, "POST", fmt.Sprintf("%s/repos/%s/%s/git/tags", GITHUB_BASE_URL, orgName, repoName), bytes.NewReader(jsonBody))
	if err != nil {
		return model.RefResponse{}, err
	}
	var tagCommitResponse refTagResponse
	if err := json.Unmarshal(tagCommitSHAResponse, &tagCommitResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the tag commit sha response body")
		return model.RefResponse{}, err
	}

	tagNewRequest := model.BranchRequest{Ref: "refs/tags/" + tagName, SHA: tagCommitResponse.SHA}
	tagByteRequest, err := json.Marshal(tagNewRequest)
	if err != nil {
		return model.RefResponse{}, err
	}
	tagByteResponse, err := invokeAPI(ctx, "POST", fmt.Sprintf(UPDATE_REF_URI, GITHUB_BASE_URL, orgName, repoName), bytes.NewReader(tagByteRequest))
	if err != nil {
		return model.RefResponse{}, err
	}
	var tagResponse model.RefResponse
	if err := json.Unmarshal(tagByteResponse, &tagResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the tag response body")
		return model.RefResponse{}, err
	}

	return tagResponse, nil
}

func ListTags(ctx *appcontext.Context, orgName, repoName string) ([]model.TagResponse, error) {
	var allTags []model.TagResponse
	url := fmt.Sprintf("%s/repos/%s/%s/tags?per_page=100", GITHUB_BASE_URL, orgName, repoName)

	for url != "" {
		resp, err := invokeAPIFull(context.Background(), ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		var page []model.TagResponse
		if err := json.Unmarshal(resp.Body, &page); err != nil {
			logger.Glog.Error().Err(err).Msg("Error in unmarshal the tags response body")
			return nil, err
		}
		allTags = append(allTags, page...)
		url = parseLinkNext(resp.LinkHeader)
	}
	return allTags, nil
}

func DeleteTag(ctx *appcontext.Context, orgName, repoName, tagName string) (bool, error) {
	_, err := invokeAPI(ctx, "DELETE", fmt.Sprintf("%s/repos/%s/%s/git/refs/tags/%s", GITHUB_BASE_URL, orgName, repoName, tagName), nil)
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
	prResponseByte, err := invokeAPI(ctx, "POST", fmt.Sprintf("%s/repos/%s/%s/pulls", GITHUB_BASE_URL, orgName, repoName), bytes.NewReader(jsonBody))
	if err != nil {
		return model.PullRequestResponse{}, err
	}
	var prResponse model.PullRequestResponse
	if err := json.Unmarshal(prResponseByte, &prResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the new PR response body")
		return model.PullRequestResponse{}, err
	}
	return prResponse, nil
}

func GetPullRequestInfo(ctx *appcontext.Context, orgName, repoName string, prNumber int) (model.PullRequestResponse, error) {
	prResponseByte, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/pulls/%d", GITHUB_BASE_URL, orgName, repoName, prNumber), nil)
	if err != nil {
		return model.PullRequestResponse{}, err
	}
	var prResponse model.PullRequestResponse
	if err := json.Unmarshal(prResponseByte, &prResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the PR response body while getting the PR info")
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
	_, err = invokeAPI(ctx, "POST", fmt.Sprintf("%s/repos/%s/%s/issues/%d/assignees", GITHUB_BASE_URL, orgName, repoName, prNumber), bytes.NewReader(jsonBody))
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
	_, err = invokeAPI(ctx, "POST", fmt.Sprintf("%s/repos/%s/%s/pulls/%d/requested_reviewers", GITHUB_BASE_URL, orgName, repoName, prNumber), bytes.NewReader(jsonBody))
	if err != nil {
		return nil, err
	}
	return fmt.Sprintf("Reviewers %s added successfully", reviewers), nil
}

func ListPullRequests(ctx *appcontext.Context, orgName, repoName string, baseRef, headRef string, all bool) ([]model.PullRequestResponse, error) {
	apiURL := fmt.Sprintf("%s/repos/%s/%s/pulls?per_page=100", GITHUB_BASE_URL, orgName, repoName)
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
			logger.Glog.Error().Err(err).Msg("Error in unmarshal the PR response body")
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
	prResponseByte, err := invokeAPI(ctx, "PATCH", fmt.Sprintf("%s/repos/%s/%s/pulls/%d", GITHUB_BASE_URL, orgName, repoName, prNumber), bytes.NewReader(jsonBody))
	if err != nil {
		return model.PullRequestResponse{}, err
	}
	var prResponse model.PullRequestResponse
	if err := json.Unmarshal(prResponseByte, &prResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the PR response body")
		return model.PullRequestResponse{}, err
	}
	return prResponse, nil
}

func ListPullRequestReviews(ctx *appcontext.Context, orgName, repoName string, prNumber int) ([]model.ReviewPullRequestResponse, error) {
	reviewResponseByte, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews", GITHUB_BASE_URL, orgName, repoName, prNumber), nil)
	if err != nil {
		return nil, err
	}
	var reviewResponses []model.ReviewPullRequestResponse
	if err := json.Unmarshal(reviewResponseByte, &reviewResponses); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the Review PR response body")
		return nil, err
	}
	return reviewResponses, nil
}

func GetPullRequestFiles(ctx *appcontext.Context, orgName, repoName string, prNumber int) ([]model.PullRequestFile, error) {
	filesResponseByte, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/pulls/%d/files", GITHUB_BASE_URL, orgName, repoName, prNumber), nil)
	if err != nil {
		return nil, err
	}
	var files []model.PullRequestFile
	if err := json.Unmarshal(filesResponseByte, &files); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the PR files response body")
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
	reviewResponseByte, err := invokeAPI(ctx, "POST", fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews", GITHUB_BASE_URL, orgName, repoName, prNumber), bytes.NewReader(jsonBody))
	if err != nil {
		return model.ReviewPullRequestResponse{RepositoryName: repoName, PRNumber: prNumber}, err
	}
	logger.Flog.Info().Str("org", orgName).Str("repo", repoName).Int("pr", prNumber).Str("event", action).Msgf("PR Review successfully")
	var reviewResponse model.ReviewPullRequestResponse
	if err := json.Unmarshal(reviewResponseByte, &reviewResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the Review PR response body")
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
	mergeResponseByte, err := invokeAPI(ctx, "PUT", fmt.Sprintf("%s/repos/%s/%s/pulls/%d/merge", GITHUB_BASE_URL, orgName, repoName, prNumber), bytes.NewReader(jsonBody))
	if err != nil {
		return model.MergeResponse{}, err
	}
	logger.Flog.Info().Str("org", orgName).Str("repo", repoName).Int("pr", prNumber).Msgf("PR Merged successfully")
	var mergeResponse model.MergeResponse
	if err := json.Unmarshal(mergeResponseByte, &mergeResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the Merge response body")
		return model.MergeResponse{}, err
	}
	return mergeResponse, nil
}

func UpdateProtectedBranch(ctx *appcontext.Context, orgName, repoName, branchName string, payload []byte) (model.ProtectedBranch, error) {
	response, err := invokeAPI(ctx, "PUT", fmt.Sprintf(PROTECTED_BRANCH_URI, GITHUB_BASE_URL, orgName, repoName, branchName), bytes.NewReader(payload))
	if err != nil {
		return model.ProtectedBranch{RepositoryName: repoName}, err
	}
	var branch model.ProtectedBranch
	if err := json.Unmarshal(response, &branch); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the branch response body")
		return model.ProtectedBranch{RepositoryName: repoName}, err
	}
	branch.RepositoryName = repoName
	branch.Type = "Branch Protection"
	return branch, nil
}

func DeleteProtectedBranch(ctx *appcontext.Context, orgName, repoName, branchName string) (bool, error) {
	_, err := invokeAPI(ctx, "DELETE", fmt.Sprintf(PROTECTED_BRANCH_URI, GITHUB_BASE_URL, orgName, repoName, branchName), nil)
	if err != nil {
		return false, err
	}
	return true, nil
}

func ListCommits(ctx *appcontext.Context, orgName, repoName, branchName string, noOfDays int) ([]model.CommitResponse, error) {
	currentTime := time.Now()
	midnight := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, 0, time.Local)
	midnight = midnight.AddDate(0, 0, -noOfDays)
	since := midnight.Format("2006-01-02T15:04:05Z")

	var allCommits []model.CommitResponse
	url := fmt.Sprintf("%s/repos/%s/%s/commits?per_page=100&since=%s&sha=%s", GITHUB_BASE_URL, orgName, repoName, since, branchName)

	for url != "" {
		resp, err := invokeAPIFull(context.Background(), ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}
		var page []model.CommitResponse
		if err := json.Unmarshal(resp.Body, &page); err != nil {
			logger.Glog.Error().Err(err).Msg("Error in unmarshal the commits response body")
			return nil, err
		}
		allCommits = append(allCommits, page...)
		url = parseLinkNext(resp.LinkHeader)
	}
	return allCommits, nil
}

func GetCommitInfo(ctx *appcontext.Context, orgName, repoName, commitSha string) (model.CommitResponse, error) {
	response, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/commits/%s", GITHUB_BASE_URL, orgName, repoName, commitSha), nil)
	if err != nil {
		return model.CommitResponse{}, err
	}
	var commit model.CommitResponse
	if err := json.Unmarshal(response, &commit); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the commit response body")
		return model.CommitResponse{}, err
	}
	return commit, nil
}

func GetCommitCheckRuns(ctx *appcontext.Context, orgName, repoName, commitSha string) (model.CheckRunResponse, error) {
	response, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/commits/%s/check-runs", GITHUB_BASE_URL, orgName, repoName, commitSha), nil)
	if err != nil {
		return model.CheckRunResponse{}, err
	}
	var checkRuns model.CheckRunResponse
	if err := json.Unmarshal(response, &checkRuns); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the check runs response body")
		return model.CheckRunResponse{}, err
	}
	return checkRuns, nil
}

func ListWorkflowRuns(ctx *appcontext.Context, orgName, repoName, branch, status string, count int) ([]model.WorkflowRun, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs?per_page=%d", GITHUB_BASE_URL, orgName, repoName, count)
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
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the workflow runs response body")
		return nil, err
	}
	return runsResponse.WorkflowRuns, nil
}

func RerunWorkflowRun(ctx *appcontext.Context, orgName, repoName string, runID int) (bool, error) {
	_, err := invokeAPI(ctx, "POST", fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d/rerun", GITHUB_BASE_URL, orgName, repoName, runID), nil)
	if err != nil {
		return false, err
	}
	return true, nil
}

func CancelWorkflowRun(ctx *appcontext.Context, orgName, repoName string, runID int) (bool, error) {
	_, err := invokeAPI(ctx, "POST", fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d/cancel", GITHUB_BASE_URL, orgName, repoName, runID), nil)
	if err != nil {
		return false, err
	}
	return true, nil
}

func GetWorkflowRun(ctx *appcontext.Context, orgName, repoName string, runID int) (model.WorkflowRun, error) {
	response, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d", GITHUB_BASE_URL, orgName, repoName, runID), nil)
	if err != nil {
		return model.WorkflowRun{}, err
	}
	var run model.WorkflowRun
	if err := json.Unmarshal(response, &run); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the workflow run response body")
		return model.WorkflowRun{}, err
	}
	return run, nil
}

func GetWorkflowRunJobs(ctx *appcontext.Context, orgName, repoName string, runID int) ([]model.WorkflowJob, error) {
	response, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d/jobs", GITHUB_BASE_URL, orgName, repoName, runID), nil)
	if err != nil {
		return nil, err
	}
	var jobsResponse model.WorkflowJobsResponse
	if err := json.Unmarshal(response, &jobsResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the workflow jobs response body")
		return nil, err
	}
	return jobsResponse.Jobs, nil
}
