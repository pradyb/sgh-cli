package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/pkg/apperrors"
	"github.com/prady-lab/sgh-cli/pkg/context"
	logger "github.com/prady-lab/sgh-cli/utils"
)

const GITHUB_BASE_URL = "https://api.github.com"
const UPDATE_REF_URI = "%s/repos/%s/%s/git/refs"
const PROTECTED_BRANCH_URI = "%s/repos/%s/%s/branches/%s/protection"

func invokeAPI(ctx *context.Context, method, url string, reqBody io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, err
	}

	res, err := ctx.HttpClient.Send(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != 200 && res.StatusCode != 201 && res.StatusCode != 204 {
		return nil, &apperrors.GitHubError{StatusCode: res.StatusCode, Message: string(body)}
	}
	return body, nil
}

func GetReposWithOrg(ctx *context.Context, orgName string) ([]model.Repository, error) {
	response, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/orgs/%s/repos?per_page=100", GITHUB_BASE_URL, orgName), nil)
	if err != nil {
		return nil, err
	}
	var repositories []model.Repository
	if err := json.Unmarshal(response, &repositories); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the org response body")
		return nil, err
	}
	return repositories, nil

}

type refResponse struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

func CreateNewBranch(ctx *context.Context, orgName, repoName, newBranchName, refBranchName string) (model.RefResponse, error) {
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

func CreateNewBranchFromCommit(ctx *context.Context, orgName, repoName, newBranchName, commitSHA string) (model.RefResponse, error) {
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

func DeleteBranch(ctx *context.Context, orgName, repoName, branchName string) (bool, error) {
	_, err := invokeAPI(ctx, "DELETE", fmt.Sprintf("%s/repos/%s/%s/git/refs/heads/%s", GITHUB_BASE_URL, orgName, repoName, branchName), nil)
	if err != nil {
		return false, err
	}
	return true, nil
}

type refTagResponse struct {
	SHA string `json:"sha"`
}

func CreateNewTag(ctx *context.Context, orgName, repoName, tagName, refBranchName, message string) (model.RefResponse, error) {
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

func DeleteTag(ctx *context.Context, orgName, repoName, tagName string) (bool, error) {
	_, err := invokeAPI(ctx, "DELETE", fmt.Sprintf("%s/repos/%s/%s/git/refs/tags/%s", GITHUB_BASE_URL, orgName, repoName, tagName), nil)
	if err != nil {
		return false, err
	}
	return true, nil
}

func CreateNewPullRequest(ctx *context.Context, orgName, repoName, title, body, baseRef, headRef string) (model.PullRequestResponse, error) {
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

func GetPullRequestInfo(ctx *context.Context, orgName, repoName string, prNumber int) (model.PullRequestResponse, error) {
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

func AddIssueAssignees(ctx *context.Context, orgName, repoName string, prNumber int, assignees []string) (interface{}, error) {
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

func AddReviewers(ctx *context.Context, orgName, repoName string, prNumber int, reviewers []string) (interface{}, error) {
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

func ListPullRequests(ctx *context.Context, orgName, repoName string, baseRef, headRef string, all bool) ([]model.PullRequestResponse, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?per_page=50", GITHUB_BASE_URL, orgName, repoName)
	if baseRef != "" {
		url = fmt.Sprintf("%s&base=%s", url, baseRef)
	}
	if headRef != "" {
		url = fmt.Sprintf("%s&head=%s:%s", url, orgName, headRef)
	}
	if all {
		url = fmt.Sprintf("%s&state=all", url)
	}
	response, err := invokeAPI(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	var prResponses []model.PullRequestResponse
	if err := json.Unmarshal(response, &prResponses); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the PR response body")
		return nil, err
	}
	return prResponses, nil
}

func UpdatePullRequest(ctx *context.Context, orgName, repoName string, prNumber int, state string) (model.PullRequestResponse, error) {
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

func ListPullRequestReviews(ctx *context.Context, orgName, repoName string, prNumber int) ([]model.ReviewPullRequestResponse, error) {
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

func ReviewPullRequest(ctx *context.Context, orgName, repoName string, prNumber int, action, body string) (model.ReviewPullRequestResponse, error) {
	reviewRequest := map[string]interface{}{
		"event": strings.ToUpper(action),
		"body":  body,
	}
	jsonBody, err := json.Marshal(reviewRequest)
	if err != nil {
		return model.ReviewPullRequestResponse{RepositoryName: repoName, PRNumber: prNumber}, err
	}
	reviewResponseByte, err := invokeAPI(ctx, "PUT", fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews", GITHUB_BASE_URL, orgName, repoName, prNumber), bytes.NewReader(jsonBody))
	if err != nil {
		return model.ReviewPullRequestResponse{RepositoryName: repoName, PRNumber: prNumber}, err
	}
	var reviewResponse model.ReviewPullRequestResponse
	if err := json.Unmarshal(reviewResponseByte, &reviewResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the Review PR response body")
		return model.ReviewPullRequestResponse{RepositoryName: repoName, PRNumber: prNumber}, err
	}
	reviewResponse.RepositoryName = repoName
	reviewResponse.PRNumber = prNumber
	return reviewResponse, nil
}

func MergePullRequest(ctx *context.Context, orgName, repoName string, prNumber int, title, body string) (model.MergeResponse, error) {
	mergeRequest := map[string]interface{}{
		"commit_title":   title,
		"commit_message": body,
	}
	jsonBody, err := json.Marshal(mergeRequest)
	if err != nil {
		return model.MergeResponse{}, err
	}
	mergeResponseByte, err := invokeAPI(ctx, "PUT", fmt.Sprintf("%s/repos/%s/%s/pulls/%d/merge", GITHUB_BASE_URL, orgName, repoName, prNumber), bytes.NewReader(jsonBody))
	if err != nil {
		return model.MergeResponse{}, err
	}
	var mergeResponse model.MergeResponse
	if err := json.Unmarshal(mergeResponseByte, &mergeResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the Merge response body")
		return model.MergeResponse{}, err
	}
	return mergeResponse, nil
}

func ListProtectedBranches(ctx *context.Context, orgName, repoName, branchName string) (model.ProtectedBranch, error) {
	response, err := invokeAPI(ctx, "GET", fmt.Sprintf(PROTECTED_BRANCH_URI, GITHUB_BASE_URL, orgName, repoName, branchName), nil)
	if err != nil {
		return model.ProtectedBranch{}, err
	}
	var branch model.ProtectedBranch
	if err := json.Unmarshal(response, &branch); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the branches response body")
		return model.ProtectedBranch{}, err
	}
	branch.RepositoryName = repoName
	return branch, nil
}

func UpdateProtectedBranch(ctx *context.Context, orgName, repoName, branchName string, payload []byte) (model.ProtectedBranch, error) {
	response, err := invokeAPI(ctx, "PUT", fmt.Sprintf(PROTECTED_BRANCH_URI, GITHUB_BASE_URL, orgName, repoName, branchName), bytes.NewReader(payload))
	if err != nil {
		return model.ProtectedBranch{}, err
	}
	var branch model.ProtectedBranch
	if err := json.Unmarshal(response, &branch); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the branch response body")
		return model.ProtectedBranch{}, err
	}
	branch.RepositoryName = repoName
	return branch, nil
}

func DeleteProtectedBranch(ctx *context.Context, orgName, repoName, branchName string) (bool, error) {
	_, err := invokeAPI(ctx, "DELETE", fmt.Sprintf(PROTECTED_BRANCH_URI, GITHUB_BASE_URL, orgName, repoName, branchName), nil)
	if err != nil {
		return false, err
	}
	return true, nil
}

func ListCommits(ctx *context.Context, orgName, repoName, branchName string, noOfDays int) ([]model.CommitResponse, error) {
	currentTime := time.Now()
	midnight := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day(), 0, 0, 0, 0, time.Local)
	midnight = midnight.AddDate(0, 0, -noOfDays)
	since := midnight.Format("2006-01-02T15:04:05Z")
	response, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/commits?since=%s&sha=%s", GITHUB_BASE_URL, orgName, repoName, since, branchName), nil)
	if err != nil {
		return nil, err
	}
	var commits []model.CommitResponse
	if err := json.Unmarshal(response, &commits); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the commits response body")
		return nil, err
	}
	return commits, nil
}

func GetCommitInfo(ctx *context.Context, orgName, repoName, commitSha string) (model.CommitResponse, error) {
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

func GetCommitCheckRuns(ctx *context.Context, orgName, repoName, commitSha string) (model.CheckRunResponse, error) {
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
