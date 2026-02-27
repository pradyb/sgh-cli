package pr

import (
	"fmt"
	"strings"
	"sync"

	"github.com/shurcooL/githubv4"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/commit"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
)

// PullRequestRequest contains parameters for creating a pull request.
type PullRequestRequest struct {
	OrgName  string
	RepoName string
	BaseRef  string
	HeadRef  string
	Title    string
	Body     string
}

// PRDetailsRequest contains parameters for getting PR details.
type PRDetailsRequest struct {
	OrgName  string
	RepoName string
	PRNumber int
	LastSHA  string
}

// PRReviewRequest contains parameters for reviewing a pull request.
type PRReviewRequest struct {
	OrgName  string
	RepoName string
	PRNumber int
	Event    string
	Body     string
}

// PRMergeRequest contains parameters for merging a pull request.
type PRMergeRequest struct {
	OrgName  string
	RepoName string
	PRNumber int
	Title    string
	Body     string
}

// PRUpdateRequest contains parameters for updating a pull request.
type PRUpdateRequest struct {
	OrgName  string
	RepoName string
	PRNumber int
	State    string
}

func CreateNewPullRequest(ctx *context.Context, prRequest PRRequest) []model.PullRequestResponse {
	responses := make([]model.PullRequestResponse, 0)

	processor.ProcessRepositoriesOperation(ctx, prRequest.OrgName, prRequest.RepoNames, prRequest.ExcludeRepoNames, processor.OperationCreatePullRequest,
		func(ctx *context.Context, orgName, repoName string) (model.PullRequestResponse, error) {
			request := PullRequestRequest{
				OrgName:  orgName,
				RepoName: repoName,
				BaseRef:  prRequest.BaseRef,
				HeadRef:  prRequest.HeadRef,
				Title:    prRequest.Title,
				Body:     prRequest.Body,
			}
			return CreateNewPullRequestForRepo(ctx, request, false)
		},
		func(repoName string, result processor.RepoOperationResult[model.PullRequestResponse]) {
			responses = append(responses, result.Result)
		},
		func(repoName string, err error) {
			responses = append(responses, model.PullRequestResponse{ErrorMessage: fmt.Sprintf("failed to create pull request: %v", err)})
		})
	return responses
}

func CreateNewPullRequestForRepo(ctx *context.Context, request PullRequestRequest, fetchPR bool) (model.PullRequestResponse, error) {
	actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(request.OrgName, []string{request.RepoName})
	if len(actualRepoNames) == 0 {
		return model.PullRequestResponse{}, fmt.Errorf("repository not found: %s", request.RepoName)
	}

	response, err := service.CreateNewPullRequest(ctx, request.OrgName, actualRepoNames[0], request.Title, request.Body, request.BaseRef, request.HeadRef)
	if err != nil {
		return model.PullRequestResponse{}, fmt.Errorf("failed to create pull request: %w", err)
	}

	return response, nil
}

type PRRequest struct {
	OrgName          string
	RepoNames        []string
	ExcludeRepoNames []string
	BaseRef          string
	HeadRef          string
	LastCount        int
	Author           string
	Assignee         string
	Reviewer         string
	Label            string
	Since            string
	All              bool
	IsInteractive    bool
	Title            string
	Body             string
}

func ListPullRequests(ctx *context.Context, prRequest PRRequest) []model.PullRequestResponse {
	responses := make([]model.PullRequestResponse, 0)

	if len(prRequest.RepoNames) <= 1 {
		// Invoke via GraphQL
		if !prRequest.IsInteractive {
			logger.Flog.Info().Msgf("Invoking GraphQL to list pull requests for %s", prRequest.OrgName)
		}

		queryString := getSearchQuery(ctx, prRequest)

		variables := map[string]any{
			"queryString": githubv4.String(queryString),
			"prCursor":    (*githubv4.String)(nil),
			"lastCount":   githubv4.Int(prRequest.LastCount),
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
				State:            pr.State,
				MergeStateStatus: pr.MergeStateStatus,
			}

			prResponse.Assignees = populateAssignees(pr.Assignees)
			prResponse.Reviewers = populateReviewers(pr.ReviewRequests)

			responses = append(responses, prResponse)
		}
		return responses
	} else {
		processor.ProcessRepositoriesOperation(ctx, prRequest.OrgName, prRequest.RepoNames, prRequest.ExcludeRepoNames, processor.OperationListPullRequest,
			func(ctx *context.Context, orgName, repoName string) ([]model.PullRequestResponse, error) {
				return service.ListPullRequests(ctx, orgName, repoName, prRequest.BaseRef, prRequest.HeadRef, prRequest.All)
			},
			func(repoName string, result processor.RepoOperationResult[[]model.PullRequestResponse]) {
				responses = append(responses, result.Result...)
			},
			func(repoName string, err error) {
				responses = append(responses, model.PullRequestResponse{ErrorMessage: fmt.Sprintf("failed to list pull requests: %v", err)})
			})
		return responses
	}
}

func getSearchQuery(ctx *context.Context, prRequest PRRequest) string {
	var queryString string
	if len(prRequest.RepoNames) == 1 {
		actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(prRequest.OrgName, prRequest.RepoNames)
		logger.Flog.Info().Str("repos", strings.Join(actualRepoNames, ",")).Msgf("Listing Pull Requests for selected repositories in %s", prRequest.OrgName)
		queryString = "repo:" + prRequest.OrgName + "/" + actualRepoNames[0]
	} else {
		queryString = "org:" + prRequest.OrgName
	}
	queryString = queryString + " type:pr state:open sort:created-desc"
	if prRequest.BaseRef != "" {
		queryString = queryString + " base:" + prRequest.BaseRef
	}
	if prRequest.HeadRef != "" {
		queryString = queryString + " head:" + prRequest.HeadRef
	}
	if prRequest.Author != "" {
		queryString = queryString + " author:" + prRequest.Author
	}
	if prRequest.Assignee != "" {
		queryString = queryString + " assignee:" + prRequest.Assignee
	}
	if prRequest.Reviewer != "" {
		queryString = queryString + " review-requested:" + prRequest.Reviewer
	}
	if prRequest.Label != "" {
		queryString = queryString + " label:" + prRequest.Label
	}
	if prRequest.Since != "" {
		queryString = queryString + " created:>=" + prRequest.Since
	}
	return queryString
}

func ReviewPullRequest(ctx *context.Context, req PRReviewRequest) model.ReviewPullRequestResponse {
	response, err := service.ReviewPullRequest(ctx, req.OrgName, req.RepoName, req.PRNumber, req.Event, req.Body)
	if err != nil {
		return model.ReviewPullRequestResponse{ErrorMessage: fmt.Sprintf("failed to review pull request: %v", err)}
	}
	return response
}

func ListPullRequestReviews(ctx *context.Context, orgName string, repoName string, prNumber int) []model.ReviewPullRequestResponse {
	response, err := service.ListPullRequestReviews(ctx, orgName, repoName, prNumber)
	if err != nil {
		return []model.ReviewPullRequestResponse{{ErrorMessage: fmt.Sprintf("failed to list pull request reviews: %v", err)}}
	}
	return response
}

func GetPullRequestFiles(ctx *context.Context, orgName string, repoName string, prNumber int) model.PullRequestFilesResponse {
	response, err := service.GetPullRequestFiles(ctx, orgName, repoName, prNumber)
	if err != nil {
		return model.PullRequestFilesResponse{RepositoryName: repoName, PRNumber: prNumber, ErrorMessage: fmt.Sprintf("failed to get pull request files: %v", err)}
	}
	return model.PullRequestFilesResponse{Files: response, RepositoryName: repoName, PRNumber: prNumber}
}

func GetPullRequestInfo(ctx *context.Context, orgName string, repoName string, prNumber int) model.PullRequestResponse {
	response, err := service.GetPullRequestInfo(ctx, orgName, repoName, prNumber)
	if err != nil {
		return model.PullRequestResponse{ErrorMessage: fmt.Sprintf("failed to get pull request info: %v", err)}
	}
	return response
}

func UpdatePullRequest(ctx *context.Context, req PRUpdateRequest) model.PullRequestResponse {
	response, err := service.UpdatePullRequest(ctx, req.OrgName, req.RepoName, req.PRNumber, req.State)
	if err != nil {
		return model.PullRequestResponse{ErrorMessage: fmt.Sprintf("failed to update pull request: %v", err)}
	}
	return response
}

func MergePullRequest(ctx *context.Context, req PRMergeRequest) model.MergeResponse {
	actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(req.OrgName, []string{req.RepoName})
	if len(actualRepoNames) == 0 {
		return model.MergeResponse{RepositoryName: req.RepoName, ErrorMessage: "Repository not found"}
	}
	response, err := service.MergePullRequest(ctx, req.OrgName, actualRepoNames[0], req.PRNumber, req.Title, req.Body)
	if err != nil {
		return model.MergeResponse{RepositoryName: actualRepoNames[0], ErrorMessage: fmt.Sprintf("failed to merge pull request: %v", err)}
	}
	response.RepositoryName = actualRepoNames[0]
	return response
}

func GetPRDetails(ctx *context.Context, req PRDetailsRequest) (model.PullRequestResponse, model.PullRequestFilesResponse, model.CheckRunResponse, []model.ReviewPullRequestResponse) {
	var wg sync.WaitGroup
	var pullRequestResponse model.PullRequestResponse
	var pullRequestFilesResponse model.PullRequestFilesResponse
	var checkRunResponse model.CheckRunResponse
	var prReviews []model.ReviewPullRequestResponse

	wg.Add(4)
	go func() {
		defer wg.Done()
		pullRequestResponse = GetPullRequestInfo(ctx, req.OrgName, req.RepoName, req.PRNumber)
	}()
	go func() {
		defer wg.Done()
		pullRequestFilesResponse = GetPullRequestFiles(ctx, req.OrgName, req.RepoName, req.PRNumber)
	}()
	go func() {
		defer wg.Done()
		checkRunResponse = commit.GetCommitCheckRuns(ctx, commit.CommitCheckRunsRequest{
			OrgName:   req.OrgName,
			RepoName:  req.RepoName,
			CommitSHA: req.LastSHA,
		})
	}()
	go func() {
		defer wg.Done()
		prReviews = ListPullRequestReviews(ctx, req.OrgName, req.RepoName, req.PRNumber)
	}()

	wg.Wait()
	return pullRequestResponse, pullRequestFilesResponse, checkRunResponse, prReviews
}

func GetPRDetailsGraphQL(ctx *context.Context, req PRDetailsRequest) (model.PullRequestResponse, model.PullRequestFilesResponse, model.CheckRunResponse, []model.ReviewPullRequestResponse) {
	variables := map[string]any{
		"orgName":  githubv4.String(req.OrgName),
		"repoName": githubv4.String(req.RepoName),
		"prNumber": githubv4.Int(req.PRNumber),
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
		CreatedAt:        prQueryReponse.CreatedAt,
		UpdatedAt:        prQueryReponse.UpdatedAt,
		MergeAt:          prQueryReponse.MergedAt,
		MergedBy: model.User{
			Login: prQueryReponse.MergedBy.User.Login,
			Name:  prQueryReponse.MergedBy.User.Name,
		},
		ReviewComments: prQueryReponse.TotalCommentsCount,
		Comments:       prQueryReponse.Comments.TotalCount,
		Commits:        prQueryReponse.Commits.TotalCount,
		Additions:      prQueryReponse.Additions,
		Deletions:      prQueryReponse.Deletions,
		ChangedFiles:   prQueryReponse.ChangedFiles,
	}

	for _, label := range prQueryReponse.Labels.Edges {
		prResponse.Labels = append(prResponse.Labels, label.Node.Name)
	}
	prResponse.Head.Sha = prQueryReponse.HeadRefOid
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
			CommitID:    review.Node.Commit.Oid,
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
		switch reviewer.Node.RequestedReviewer.Type {
		case "User":
			reviewersList = append(reviewersList, model.Actor{
				Type: "User",
				User: model.User{
					Login: reviewer.Node.RequestedReviewer.User.Login,
					Name:  reviewer.Node.RequestedReviewer.User.Name,
				},
			})
		case "Team":
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
