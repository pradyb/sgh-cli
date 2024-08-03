package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/pkg/apperrors"
	"github.com/prady-lab/sgh-cli/pkg/context"
	logger "github.com/prady-lab/sgh-cli/utils"
)

const GITHUB_BASE_URL = "https://api.github.com"
const UPDATE_REF_URI = "%s/repos/%s/%s/git/refs"

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

func CreateNewBranch(ctx *context.Context, orgName, repoName, newBranchName, refBranchName string) (model.NewItemResponse, error) {
	commitSHAResponse, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/git/ref/heads/%s", GITHUB_BASE_URL, orgName, repoName, refBranchName), nil)
	if err != nil {
		return model.NewItemResponse{}, err
	}
	var refResponse refResponse
	if err := json.Unmarshal(commitSHAResponse, &refResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the response body")
		return model.NewItemResponse{}, err
	}

	branchRequest := model.BranchRequest{Ref: "refs/heads/" + newBranchName, SHA: refResponse.Object.SHA}
	jsonBody, err := json.Marshal(branchRequest)
	if err != nil {
		return model.NewItemResponse{}, err
	}
	branchResponseByte, err := invokeAPI(ctx, "POST", fmt.Sprintf(UPDATE_REF_URI, GITHUB_BASE_URL, orgName, repoName), bytes.NewReader(jsonBody))
	if err != nil {
		return model.NewItemResponse{}, err
	}
	var branchResponse model.NewItemResponse
	if err := json.Unmarshal(branchResponseByte, &branchResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the branch response body")
		return model.NewItemResponse{}, err
	}
	return branchResponse, nil
}

func CreateNewBranchFromCommit(ctx *context.Context, orgName, repoName, newBranchName, commitSHA string) (model.NewItemResponse, error) {
	branchRequest := model.BranchRequest{Ref: "refs/heads/" + newBranchName, SHA: commitSHA}
	jsonBody, err := json.Marshal(branchRequest)
	if err != nil {
		return model.NewItemResponse{}, err
	}
	branchResponseByte, err := invokeAPI(ctx, "POST", fmt.Sprintf(UPDATE_REF_URI, GITHUB_BASE_URL, orgName, repoName), bytes.NewReader(jsonBody))
	if err != nil {
		return model.NewItemResponse{}, err
	}
	var branchResponse model.NewItemResponse
	if err := json.Unmarshal(branchResponseByte, &branchResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the branch response body")
		return model.NewItemResponse{}, err
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

func CreateNewTag(ctx *context.Context, orgName, repoName, tagName, refBranchName, message string) (model.NewItemResponse, error) {
	refCommitSHAResponse, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/git/ref/heads/%s", GITHUB_BASE_URL, orgName, repoName, refBranchName), nil)
	if err != nil {
		return model.NewItemResponse{}, err
	}
	var refResponse refResponse
	if err := json.Unmarshal(refCommitSHAResponse, &refResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the tag response body")
		return model.NewItemResponse{}, err
	}

	tagRequest := model.TagRequest{Tag: tagName, Object: refResponse.Object.SHA, Message: message, Type: "commit", Tagger: model.Tagger{Name: ctx.Config.TaggerName(orgName), Email: ctx.Config.TaggerEmail(orgName)}}
	jsonBody, err := json.Marshal(tagRequest)
	if err != nil {
		return model.NewItemResponse{}, err
	}
	tagCommitSHAResponse, err := invokeAPI(ctx, "POST", fmt.Sprintf("%s/repos/%s/%s/git/tags", GITHUB_BASE_URL, orgName, repoName), bytes.NewReader(jsonBody))
	if err != nil {
		return model.NewItemResponse{}, err
	}
	var tagCommitResponse refTagResponse
	if err := json.Unmarshal(tagCommitSHAResponse, &tagCommitResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the tag commit sha response body")
		return model.NewItemResponse{}, err
	}

	tagNewRequest := model.BranchRequest{Ref: "refs/tags/" + tagName, SHA: tagCommitResponse.SHA}
	tagByteRequest, err := json.Marshal(tagNewRequest)
	if err != nil {
		return model.NewItemResponse{}, err
	}
	tagByteResponse, err := invokeAPI(ctx, "POST", fmt.Sprintf(UPDATE_REF_URI, GITHUB_BASE_URL, orgName, repoName), bytes.NewReader(tagByteRequest))
	if err != nil {
		return model.NewItemResponse{}, err
	}
	var tagResponse model.NewItemResponse
	if err := json.Unmarshal(tagByteResponse, &tagResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the tag response body")
		return model.NewItemResponse{}, err
	}

	return tagResponse, nil
}

func DeleteTag(ctx *context.Context, orgName, repoName, tagName string) (interface{}, error) {
	_, err := invokeAPI(ctx, "DELETE", fmt.Sprintf("%s/repos/%s/%s/git/refs/tags/%s", GITHUB_BASE_URL, orgName, repoName, tagName), nil)
	if err != nil {
		return nil, err
	}
	return fmt.Sprintf("Tag %s deleted successfully", tagName), nil
}

func CreateNewPullRequest(ctx *context.Context, orgName, repoName, title, body, targetBranch, sourceBranch string) (model.PRResponse, error) {
	prRequest := map[string]interface{}{
		"title": title,
		"body":  body,
		"head":  sourceBranch,
		"base":  targetBranch,
	}
	jsonBody, err := json.Marshal(prRequest)
	if err != nil {
		return model.PRResponse{}, err
	}
	prResponseByte, err := invokeAPI(ctx, "POST", fmt.Sprintf("%s/repos/%s/%s/pulls", GITHUB_BASE_URL, orgName, repoName), bytes.NewReader(jsonBody))
	if err != nil {
		return model.PRResponse{}, err
	}
	var prResponse model.PRResponse
	if err := json.Unmarshal(prResponseByte, &prResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the PR response body")
		return model.PRResponse{}, err
	}
	return prResponse, nil
}

func GetPullRequestInfo(ctx *context.Context, orgName, repoName string, prNumber int) (model.PRResponse, error) {
	prResponseByte, err := invokeAPI(ctx, "GET", fmt.Sprintf("%s/repos/%s/%s/pulls/%d", GITHUB_BASE_URL, orgName, repoName, prNumber), nil)
	if err != nil {
		return model.PRResponse{}, err
	}
	var prResponse model.PRResponse
	if err := json.Unmarshal(prResponseByte, &prResponse); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in unmarshal the PR response body")
		return model.PRResponse{}, err
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
