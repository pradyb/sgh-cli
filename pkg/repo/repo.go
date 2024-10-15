package repo

import (
	"sort"
	"strings"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/config"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/shurcooL/githubv4"
)

func GetReposForOrg(ctx *context.Context, orgName string, all bool) ([]model.Repository, error) {
	var queryString string
	queryString = "org:" + orgName

	if ctx.Config.IsOrganizationPresent(orgName) {
		includes := ctx.Config.IncludePatterns(orgName)
		if (len(includes)) == 1 {
			queryString = queryString + " " + strings.ReplaceAll(includes[0], "*", "") + " in:name"
		}
	}

	variables := map[string]interface{}{
		"queryString": githubv4.String(queryString),
		"repoCursor":  (*githubv4.String)(nil),
	}

	repositories := make([]model.Repository, 0)
	for {
		var searchRepositoriesQuery model.SearchRepositoriesQuery
		err := service.Query(ctx, &searchRepositoriesQuery, variables)
		if err != nil {
			return nil, err
		}

		for _, edge := range searchRepositoriesQuery.Search.Edges {
			repo := edge.Node.Repository
			repositories = append(repositories, model.Repository{
				Name:                  repo.Name,
				HTMLUrl:               repo.Url,
				SSHUrl:                repo.SSHUrl,
				Description:           repo.Description,
				DefaultBranch:         repo.DefaultBranchRef.Name,
				Language:              repo.PrimaryLanguage.Name,
				Private:               repo.IsPrivate,
				OpenPullRequestsCount: repo.PullRequests.TotalCount,
				//OpenIssuesCount:       repo.Issues.TotalCount,
			})
		}
		variables["repoCursor"] = githubv4.String(searchRepositoriesQuery.Search.PageInfo.EndCursor)
		logger.Flog.Info().Msgf("Next page details %t %s", searchRepositoriesQuery.Search.PageInfo.HasNextPage, searchRepositoriesQuery.Search.PageInfo.EndCursor)

		if !searchRepositoriesQuery.Search.PageInfo.HasNextPage {
			break
		}
	}

	sort.Slice(repositories, func(i, j int) bool {
		return repositories[i].Name < repositories[j].Name
	})

	if all {
		return repositories, nil
	}
	if ctx.Config.IsOrganizationPresent(orgName) {
		filteredRepo, err := filteredRepos(ctx, repositories, orgName)
		saveRepositoryNamesForFuzzySearch(ctx, orgName, filteredRepo)
		return filteredRepo, err
	}
	return make([]model.Repository, 0), nil
}

func GetSelectedRepoNames(ctx *context.Context, orgName string) ([]string, error) {
	repos, err := GetReposForOrg(ctx, orgName, false)
	if err != nil {
		return nil, err
	}

	repoNames := make([]string, 0)
	for _, repo := range repos {
		repoNames = append(repoNames, repo.Name)
	}
	return repoNames, nil
}

func filteredRepos(ctx *context.Context, repositories []model.Repository, orgName string) ([]model.Repository, error) {
	var filteredRepos []model.Repository
	for _, repo := range repositories {
		if ctx.Config.CanSelectRepositoryForProcessing(orgName, repo.Name) {
			filteredRepos = append(filteredRepos, repo)
		}
	}
	return filteredRepos, nil
}

func saveRepositoryNamesForFuzzySearch(ctx *context.Context, orgName string, repositories []model.Repository) {
	var repoNames []string
	for _, repo := range repositories {
		repoNames = append(repoNames, repo.Name)
	}
	config.SaveRepositoryNamesForFuzzySearch(ctx, orgName, repoNames)
}
