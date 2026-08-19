// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

// Package repo provides functions for interacting with GitHub repositories, including listing,
// filtering, and retrieving repository details for organizations.
package repo

import (
	"fmt"
	"slices"
	"sort"

	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/pkg/config"
	"github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/logger"
	"github.com/shurcooL/githubv4"
)

// ownerQualifier returns "user" or "org" for the given owner name.
// It checks the config cache first; on a miss it calls the GitHub API and
// writes the result back to config so subsequent calls are free.
func ownerQualifier(ctx *context.Context, orgName string) string {
	if cached := ctx.Config.OwnerTypeFor(orgName); cached != "" {
		if cached == "User" {
			return "user"
		}
		return "org"
	}
	ownerType, err := service.GetOwnerType(ctx, orgName)
	if err != nil {
		return "org"
	}
	ctx.Config.SetOwnerType(orgName, ownerType)
	if saveErr := ctx.Config.Save(); saveErr != nil {
		logger.Glog.Warn().Err(saveErr).Msg("failed to cache owner type")
	}
	if ownerType == "User" {
		return "user"
	}
	return "org"
}

// GetReposForOrg retrieves repositories for the given organization, optionally filtering by config.
// If all is true, returns all repositories; otherwise, applies config-based filtering.
func GetReposForOrg(ctx *context.Context, orgName string, all bool) ([]model.Repository, error) {
	// Always fetch all repos for the org and apply include/exclude patterns
	// in-memory via CanSelectRepositoryForProcessing. The old approach of
	// injecting a single include pattern directly into the GitHub search query
	// was incorrect (patterns are regex, not globs) and only worked for exactly
	// one pattern, causing inconsistent behaviour.
	queryString := ownerQualifier(ctx, orgName) + ":" + orgName

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
				// OpenIssuesCount:       repo.Issues.TotalCount,
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
	logger.Glog.Warn().Str("org", orgName).Msg("owner not in config — add it with: sgh config add org " + orgName)
	return make([]model.Repository, 0), nil
}

// SearchRepos searches repositories within an organization using GitHub's search qualifiers.
func SearchRepos(ctx *context.Context, orgName, query, language, topic string) ([]model.Repository, error) {
	queryString := ownerQualifier(ctx, orgName) + ":" + orgName

	if query != "" {
		queryString += " " + query + " in:name,description"
	}
	if language != "" {
		queryString += " language:" + language
	}
	if topic != "" {
		queryString += " topic:" + topic
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
			})
		}
		variables["repoCursor"] = githubv4.String(searchRepositoriesQuery.Search.PageInfo.EndCursor)

		if !searchRepositoriesQuery.Search.PageInfo.HasNextPage {
			break
		}
	}

	sort.Slice(repositories, func(i, j int) bool {
		return repositories[i].Name < repositories[j].Name
	})

	return repositories, nil
}

// GetSelectedRepoNames returns the names of selected repositories for the given organization.
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

// filteredRepos filters the given repositories for the organization using config rules.
func filteredRepos(ctx *context.Context, repositories []model.Repository, orgName string) ([]model.Repository, error) {
	var filteredRepos []model.Repository
	for _, repo := range repositories {
		if ctx.Config.CanSelectRepositoryForProcessing(orgName, repo.Name) {
			filteredRepos = append(filteredRepos, repo)
		}
	}
	return filteredRepos, nil
}

// ArchiveRepos archives or unarchives repositories in bulk.
func ArchiveRepos(ctx *context.Context, orgName string, repoNames, excludeRepoNames []string, archive bool) []model.RefUIResponse {
	action := "ARCHIVE"
	verb := "archived"
	if !archive {
		action = "UNARCHIVE"
		verb = "unarchived"
	}

	resolved := resolveRepoList(ctx, orgName, repoNames, excludeRepoNames)
	responses := make([]model.RefUIResponse, 0, len(resolved))
	for _, repoName := range resolved {
		err := service.UpdateRepoArchived(ctx, orgName, repoName, archive)
		if err != nil {
			responses = append(responses, model.CreateNewCommonResponse(repoName, repoName, action, "", fmt.Sprintf("failed to %s: %v", verb, err)))
		} else {
			responses = append(responses, model.CreateNewCommonResponse(repoName, repoName, action, fmt.Sprintf("Repository %s", verb), ""))
		}
	}
	return responses
}

// SetRepoVisibility changes repository visibility in bulk.
func SetRepoVisibility(ctx *context.Context, orgName string, repoNames, excludeRepoNames []string, visibility string) []model.RefUIResponse {
	resolved := resolveRepoList(ctx, orgName, repoNames, excludeRepoNames)
	responses := make([]model.RefUIResponse, 0, len(resolved))
	for _, repoName := range resolved {
		err := service.UpdateRepoVisibility(ctx, orgName, repoName, visibility)
		if err != nil {
			responses = append(responses, model.CreateNewCommonResponse(repoName, repoName, "SET_VISIBILITY", "", fmt.Sprintf("failed to set visibility: %v", err)))
		} else {
			responses = append(responses, model.CreateNewCommonResponse(repoName, repoName, "SET_VISIBILITY", fmt.Sprintf("Visibility set to %s", visibility), ""))
		}
	}
	return responses
}

// resolveRepoList returns the list of repos to process, applying the same fuzzy
// resolution and config-based filtering used by PR/branch operations.
func resolveRepoList(ctx *context.Context, orgName string, repos, excludeRepos []string) []string {
	repoNames := make([]string, 0)
	if len(repos) == 0 {
		orgRepoNames, err := GetSelectedRepoNames(ctx, orgName)
		if err != nil {
			logger.Glog.Error().Err(err).Str("org", orgName).Msg("failed to resolve repository names")
			return nil
		}
		repoNames = append(repoNames, orgRepoNames...)
	} else {
		repoNames = append(repoNames, ctx.Config.ActualRepositoryNamesUsingFzf(orgName, repos)...)
	}

	// Apply config include/exclude patterns.
	patternFiltered := make([]string, 0, len(repoNames))
	for _, repoName := range repoNames {
		if ctx.Config.CanSelectRepositoryForProcessing(orgName, repoName) {
			patternFiltered = append(patternFiltered, repoName)
		}
	}
	repoNames = patternFiltered

	// Apply explicit --exclude-repo with fuzzy resolution.
	filteredRepoNames := make([]string, 0, len(repoNames))
	actualExcludeRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, excludeRepos)
	for _, repoName := range repoNames {
		if !slices.Contains(actualExcludeRepoNames, repoName) {
			filteredRepoNames = append(filteredRepoNames, repoName)
		}
	}
	return filteredRepoNames
}

// saveRepositoryNamesForFuzzySearch saves repository names for fuzzy search in config.
func saveRepositoryNamesForFuzzySearch(ctx *context.Context, orgName string, repositories []model.Repository) {
	var repoNames []string
	for _, repo := range repositories {
		repoNames = append(repoNames, repo.Name)
	}
	config.SaveRepositoryNamesForFuzzySearch(ctx, orgName, repoNames)
}
