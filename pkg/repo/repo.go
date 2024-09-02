package repo

import (
	"sort"
	"strings"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/config"
	"github.com/prady-lab/sgh-cli/pkg/context"
	logger "github.com/prady-lab/sgh-cli/utils"
	"github.com/shurcooL/githubv4"
)

var q struct {
	Search struct {
		RepositoryCount int
		PageInfo        struct {
			EndCursor   string
			HasNextPage bool
		}
		Edges []struct {
			Node struct {
				Repository struct {
					Name             string
					NameWithOwner    string
					Url              string
					SSHUrl           string
					Description      string
					IsPrivate        bool
					IsArchived       bool
					IsDisabled       bool
					DefaultBranchRef struct {
						Name string
					}
					PrimaryLanguage struct {
						Name string
					}
					PullRequests struct {
						TotalCount int
					} `graphql:"pullRequests(first:1, states: OPEN)"`
					Issues struct {
						TotalCount int
					} `graphql:"issues(first:1, states: OPEN)"`
				} `graphql:"... on Repository"`
			}
		}
	} `graphql:"search(query: $queryString, type: REPOSITORY, first: 50, after: $repoCursor)"`
}

func GetReposForOrg(ctx *context.Context, orgName string, all bool) ([]model.Repository, error) {
	var queryString string
	queryString = "org:" + orgName

	if ctx.Config.IsOrganizationPresent(orgName) {
		includes := ctx.Config.IncludePatterns(orgName)
		if (len(includes)) > 0 {
			queryString = queryString + " " + strings.ReplaceAll(includes[0], "*", "") + " in:name"
		}
	}

	variables := map[string]interface{}{
		"queryString": githubv4.String(queryString),
		"repoCursor":  (*githubv4.String)(nil),
	}

	repositories := make([]model.Repository, 0)
	for {
		err := service.Query(ctx, &q, variables)
		if err != nil {
			return nil, err
		}

		for _, edge := range q.Search.Edges {
			repo := edge.Node.Repository
			repositories = append(repositories, model.Repository{
				Name:                  repo.Name,
				HTMLUrl:               repo.Url,
				SSHUrl:                repo.SSHUrl,
				Description:           repo.Description,
				DefaultBranch:         repo.DefaultBranchRef.Name,
				Language:              repo.PrimaryLanguage.Name,
				OpenIssuesCount:       repo.Issues.TotalCount,
				OpenPullRequestsCount: repo.PullRequests.TotalCount,
			})
		}
		variables["repoCursor"] = githubv4.String(q.Search.PageInfo.EndCursor)
		logger.Flog.Info().Msgf("Next page details %t %s", q.Search.PageInfo.HasNextPage, q.Search.PageInfo.EndCursor)

		if !q.Search.PageInfo.HasNextPage {
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
