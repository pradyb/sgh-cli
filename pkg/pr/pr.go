package pr

import (
	"strings"
	"sync"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/commit"
	"github.com/prady-lab/sgh-cli/pkg/context"
	logger "github.com/prady-lab/sgh-cli/utils"
	"github.com/shurcooL/githubv4"
)

func CreateNewPullRequest(ctx *context.Context, orgName string, repoNames []string, baseRef, headRef, title, body string) []model.PullRequestResponse {
	responses := make([]model.PullRequestResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, processor.OperationCreatePullRequest,
		func(ctx *context.Context, orgName, repoName string) (model.PullRequestResponse, error) {
			return CreateNewPullRequestForRepo(ctx, orgName, repoName, baseRef, headRef, title, body, true)
		},
		func(repoName string, result processor.RepoOperationResult[model.PullRequestResponse]) {
			responses = append(responses, result.Result)
		},
		func(repoName string, err error) {
			responses = append(responses, model.PullRequestResponse{ErrorMessage: err.Error()})
		})
	return responses
}

func CreateNewPullRequestForRepo(ctx *context.Context, orgName string, repoName, baseRef, headRef, title, body string, fetchPR bool) (model.PullRequestResponse, error) {
	prResponse, err := service.CreateNewPullRequest(ctx, orgName, repoName, title, body, baseRef, headRef)
	if err != nil {
		return model.PullRequestResponse{}, err
	}
	assignees := ctx.Config.PullRequestAssignees(orgName)
	if len(assignees) > 0 {
		service.AddIssueAssignees(ctx, orgName, repoName, prResponse.PRNumber, assignees)
		service.AddReviewers(ctx, orgName, repoName, prResponse.PRNumber, assignees)
	}
	if fetchPR {
		return service.GetPullRequestInfo(ctx, orgName, repoName, prResponse.PRNumber)
	} else {
		return prResponse, nil
	}
}

func ListPullRequests(ctx *context.Context, orgName string, repoNames []string, baseRef, headRef string, all bool) []model.PullRequestResponse {
	responses := make([]model.PullRequestResponse, 0)

	if len(repoNames) <= 1 {
		// Invoke via GraphQL
		logger.Glog.Info().Msgf("Invoking GraphQL to list pull requests for %s", orgName)

		queryString := getSearchQuery(ctx, orgName, repoNames, baseRef, headRef)

		variables := map[string]interface{}{
			"queryString": githubv4.String(queryString),
			"prCursor":    (*githubv4.String)(nil),
		}

		var pullRequestQuery model.SearchPullRequestsQuery
		err := service.Query(ctx, &pullRequestQuery, variables)
		if err != nil {
			return responses
		}

		for _, edge := range pullRequestQuery.Search.Edges {
			pr := edge.Node.PullRequest
			prResponse := model.PullRequestResponse{
				PRNumber:  pr.Number,
				TitleName: pr.Title,
				Body:      pr.Body,
				HTMLUrl:   pr.Url,
				Base: model.PRBranch{
					Ref: pr.BaseRef.Name,
					Repo: model.Repository{
						Name: pr.BaseRef.Repository.Name,
					},
				},
				Head: model.PRBranch{
					Ref: pr.HeadRef.Name,
					Repo: model.Repository{
						Name: pr.HeadRef.Repository.Name,
					},
				},
				Author: model.User{
					Login: pr.Author.User.Login,
					Name:  pr.Author.User.Name,
				},
			}

			prResponse.Assignees = populateAssignees(pr.Assignees)
			prResponse.Reviewers = populateReviewers(pr.ReviewRequests)

			responses = append(responses, prResponse)
		}
		return responses
	} else {
		processor.ProcessRepositoriesOperation(ctx, orgName, repoNames, processor.OperationListPullRequest,
			func(ctx *context.Context, orgName, repoName string) ([]model.PullRequestResponse, error) {
				return service.ListPullRequests(ctx, orgName, repoName, baseRef, headRef, all)
			},
			func(repoName string, result processor.RepoOperationResult[[]model.PullRequestResponse]) {
				responses = append(responses, result.Result...)
			},
			func(repoName string, err error) {
				responses = append(responses, model.PullRequestResponse{ErrorMessage: err.Error()})
			})
		return responses
	}
}

func getSearchQuery(ctx *context.Context, orgName string, repoNames []string, baseRef string, headRef string) string {
	var queryString string
	if len(repoNames) == 1 {
		actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, repoNames)
		logger.Glog.Info().Str("repos", strings.Join(actualRepoNames, ",")).Msgf("Listing Pull Requests for selected repositories in %s", orgName)
		queryString = "repo:" + orgName + "/" + actualRepoNames[0]
	} else {
		queryString = "org:" + orgName
	}
	queryString = queryString + " type:pr state:open sort:created-desc"
	if baseRef != "" {
		queryString = queryString + " base:" + baseRef
	}
	if headRef != "" {
		queryString = queryString + " head:" + headRef
	}
	return queryString
}

func ReviewPullRequest(ctx *context.Context, orgName string, repoName string, prNumber int, event, body string) model.ReviewPullRequestResponse {
	response, err := service.ReviewPullRequest(ctx, orgName, repoName, prNumber, event, body)
	if err != nil {
		return model.ReviewPullRequestResponse{ErrorMessage: err.Error()}
	}
	return response
}

func ListPullRequestReviews(ctx *context.Context, orgName string, repoName string, prNumber int) []model.ReviewPullRequestResponse {
	response, err := service.ListPullRequestReviews(ctx, orgName, repoName, prNumber)
	if err != nil {
		return []model.ReviewPullRequestResponse{{ErrorMessage: err.Error()}}
	}
	return response
}

func GetPullRequestFiles(ctx *context.Context, orgName string, repoName string, prNumber int) model.PullRequestFilesResponse {
	response, err := service.GetPullRequestFiles(ctx, orgName, repoName, prNumber)
	if err != nil {
		return model.PullRequestFilesResponse{RepositoryName: repoName, PRNumber: prNumber, ErrorMessage: err.Error()}
	}
	return model.PullRequestFilesResponse{RepositoryName: repoName, PRNumber: prNumber, Files: response}
}

func GetPullRequestInfo(ctx *context.Context, orgName string, repoName string, prNumber int) model.PullRequestResponse {
	response, err := service.GetPullRequestInfo(ctx, orgName, repoName, prNumber)
	if err != nil {
		return model.PullRequestResponse{ErrorMessage: err.Error()}
	}
	return response
}

func UpdatePullRequest(ctx *context.Context, orgName string, repoName string, prNumber int, state string) model.PullRequestResponse {
	response, err := service.UpdatePullRequest(ctx, orgName, repoName, prNumber, state)
	if err != nil {
		return model.PullRequestResponse{ErrorMessage: err.Error()}
	}
	return response
}

func MergePullRequest(ctx *context.Context, orgName string, repoName string, prNumber int, title, body string) model.MergeResponse {
	actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, []string{repoName})
	if len(actualRepoNames) == 0 {
		return model.MergeResponse{RepositoryName: actualRepoNames[0], ErrorMessage: "Repository not found"}
	}
	response, err := service.MergePullRequest(ctx, orgName, actualRepoNames[0], prNumber, title, body)
	if err != nil {
		return model.MergeResponse{RepositoryName: actualRepoNames[0], ErrorMessage: err.Error()}
	}
	response.RepositoryName = actualRepoNames[0]
	return response
}

func GetPRDetails(ctx *context.Context, orgName string, repoName string, prNumber int, lastSha string) (model.PullRequestResponse, model.PullRequestFilesResponse, model.CheckRunResponse, []model.ReviewPullRequestResponse) {

	var wg sync.WaitGroup
	var pullRequestResponse model.PullRequestResponse
	var pullRequestFilesResponse model.PullRequestFilesResponse
	var checkRunResponse model.CheckRunResponse
	var prReviews []model.ReviewPullRequestResponse

	wg.Add(4)
	go func() {
		defer wg.Done()
		pullRequestResponse = GetPullRequestInfo(ctx, orgName, repoName, prNumber)
	}()
	go func() {
		defer wg.Done()
		pullRequestFilesResponse = GetPullRequestFiles(ctx, orgName, repoName, prNumber)
	}()
	go func() {
		defer wg.Done()
		checkRunResponse = commit.GetCommitCheckRuns(ctx, orgName, repoName, lastSha)
	}()
	go func() {
		defer wg.Done()
		prReviews = ListPullRequestReviews(ctx, orgName, repoName, prNumber)
	}()

	wg.Wait()
	return pullRequestResponse, pullRequestFilesResponse, checkRunResponse, prReviews
}

func GetPRDetailsGraphQL(ctx *context.Context, orgName string, repoName string, prNumber int, lastSha string) (model.PullRequestResponse, model.PullRequestFilesResponse, model.CheckRunResponse, []model.ReviewPullRequestResponse) {

	variables := map[string]interface{}{
		"orgName":  githubv4.String(orgName),
		"repoName": githubv4.String(repoName),
		"prNumber": githubv4.Int(prNumber),
	}

	var prResponse model.PullRequestResponse
	var pullRequestFilesResponse model.PullRequestFilesResponse
	var checkRunResponse model.CheckRunResponse
	var prReviews []model.ReviewPullRequestResponse

	var pullRequestDetailQuery model.PullRequestDetailQuery
	err := service.Query(ctx, &pullRequestDetailQuery, variables)
	if err != nil {
		return model.PullRequestResponse{ErrorMessage: err.Error()}, model.PullRequestFilesResponse{}, model.CheckRunResponse{}, nil
	}

	prQueryReponse := pullRequestDetailQuery.Organization.Repository.PullRequest
	prResponse = model.PullRequestResponse{
		PRNumber:  prQueryReponse.Number,
		TitleName: prQueryReponse.Title,
		Body:      prQueryReponse.Body,
		HTMLUrl:   prQueryReponse.Url,
		Base: model.PRBranch{
			Ref: prQueryReponse.BaseRef.Name,
			Repo: model.Repository{
				Name: prQueryReponse.BaseRef.Repository.Name,
			},
		},
		Head: model.PRBranch{
			Ref: prQueryReponse.HeadRef.Name,
			Repo: model.Repository{
				Name: prQueryReponse.HeadRef.Repository.Name,
			},
		},
		Author: model.User{
			Login: prQueryReponse.Author.User.Login,
			Name:  prQueryReponse.Author.User.Name,
		},
		ReviewDecision:   prQueryReponse.ReviewDecision,
		State:            prQueryReponse.State,
		Mergeable:        prQueryReponse.Mergeable,
		MergeStateStatus: prQueryReponse.MergeStateStatus,
		MergeAt:          prQueryReponse.MergedAt,
		MergedBy: model.User{
			Login: prQueryReponse.MergedBy.User.Name,
			Name:  prQueryReponse.MergedBy.User.Login,
		},
		ReviewComments: prQueryReponse.TotalCommentsCount,
		Comments:       prQueryReponse.Comments.TotalCount,
		Commits:        prQueryReponse.Commits.TotalCount,
		Additions:      prQueryReponse.Additions,
		Deletions:      prQueryReponse.Deletions,
		ChangedFiles:   prQueryReponse.ChangedFiles,
	}

	prResponse.Assignees = populateAssignees(prQueryReponse.Assignees)
	prResponse.Reviewers = populateReviewers(prQueryReponse.ReviewRequests)

	for _, file := range prQueryReponse.Files.Edges {
		pullRequestFilesResponse.Files = append(pullRequestFilesResponse.Files, model.PullRequestFile{
			Filename:   file.Node.Path,
			Additions:  file.Node.Additions,
			Deletions:  file.Node.Deletions,
			ChangeType: file.Node.ChangeType,
		})
	}

	for _, commit := range prQueryReponse.Commits.Edges {
		for _, checkSuite := range commit.Node.Commit.CheckSuites.Edges {
			checkRunResponse.OverallConclusion = checkSuite.Node.Conclusion
			for _, checkRun := range checkSuite.Node.CheckRuns.Edges {
				checkRunResponse.CheckRuns = append(checkRunResponse.CheckRuns, model.CheckRun{
					Status:      checkRun.Node.Status,
					Conclusion:  checkRun.Node.Conclusion,
					StartedAt:   checkRun.Node.StartedAt,
					CompletedAt: checkRun.Node.CompletedAt,
					DetailsUrl:  checkRun.Node.DetailsUrl,
					Name:        checkRun.Node.Name,
					Output: model.CheckRunOutput{
						Title:   checkRun.Node.Title,
						Summary: checkRun.Node.Summary,
						Text:    checkRun.Node.Text,
					},
				})
			}
		}
	}

	for _, review := range prQueryReponse.Reviews.Edges {
		prReviews = append(prReviews, model.ReviewPullRequestResponse{
			User: model.User{
				Login: review.Node.Author.User.Login,
				Name:  review.Node.Author.User.Name,
			},
			State:       review.Node.State,
			Body:        review.Node.Body,
			CreatedAt:   review.Node.CreatedAt,
			SubmittedAt: review.Node.SubmittedAt,
			CommitId:    review.Node.Commit.Oid,
		})
	}

	return prResponse, pullRequestFilesResponse, checkRunResponse, prReviews
}

func populateAssignees(assigness model.AssigneesFragment) []model.User {
	assignees := make([]model.User, 0)
	for _, assignee := range assigness.Edges {
		assignees = append(assignees, model.User{
			Login: assignee.Node.Login,
			Name:  assignee.Node.Name,
		})
	}
	return assignees
}

func populateReviewers(reviewers model.ReviewRequestsFragment) []model.Actor {
	reviewersList := make([]model.Actor, 0)
	for _, reviewer := range reviewers.Edges {
		if reviewer.Node.RequestedReviewer.Type == "User" {
			reviewersList = append(reviewersList, model.Actor{
				Type: "User",
				User: model.User{
					Login: reviewer.Node.RequestedReviewer.User.Login,
					Name:  reviewer.Node.RequestedReviewer.User.Name,
				},
			})
		} else if reviewer.Node.RequestedReviewer.Type == "Team" {
			reviewersList = append(reviewersList, model.Actor{
				Type: "Team",
				Team: model.OrgTeam{
					Name: reviewer.Node.RequestedReviewer.User.Name,
				},
			})
		}
	}
	return reviewersList
}
