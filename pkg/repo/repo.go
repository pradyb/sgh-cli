package repo

import (
	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/config"
	"github.com/prady-lab/sgh-cli/pkg/context"
)

func GetReposForOrg(ctx *context.Context, orgName string, all bool) ([]model.Repository, error) {

	repositories, err := service.GetReposWithOrg(ctx, orgName)
	if err != nil {
		return nil, err
	}

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
