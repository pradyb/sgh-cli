package config

import (
	"github.com/prady-lab/sgh-cli/pkg/context"
	logger "github.com/prady-lab/sgh-cli/utils"
)

func AddOrganization(ctx *context.Context, orgName string) bool {
	if !ctx.Config.AddOrganization(orgName) {
		logger.Glog.Info().Msgf("Organization %s already present", orgName)
		return true
	}
	saveConfig(ctx)
	logger.Glog.Info().Msgf("Organization %s added successfully", orgName)
	return false
}

func AddRepository(ctx *context.Context, orgName string, repoName string) bool {
	if !ctx.Config.AddRepository(orgName, repoName) {
		logger.Glog.Info().Msgf("Repository %s already present", repoName)
		return true
	}
	saveConfig(ctx)
	logger.Glog.Info().Msgf("Repository %s added successfully", repoName)
	return false
}

func AddPullRequestAssignee(ctx *context.Context, orgName string, assignee string) bool {
	if !ctx.Config.AddPullRequestAssignee(orgName, assignee) {
		logger.Glog.Info().Msgf("Pull request assignee %s already present", assignee)
		return true
	}
	saveConfig(ctx)
	logger.Glog.Info().Msgf("Pull request assignee %s added successfully", assignee)
	return false
}

func AddRepositoryPattern(ctx *context.Context, orgName string, include bool, exclude bool, pattern string) bool {
	if !ctx.Config.AddRepositoryPattern(orgName, include, exclude, pattern) {
		logger.Glog.Info().Msgf("Repository pattern already present")
		return true
	}
	saveConfig(ctx)
	logger.Glog.Info().Msgf("Repository pattern %s added successfully", pattern)
	return false
}

func SaveRepositoryNamesForFuzzySearch(ctx *context.Context, orgName string, repoNames []string) {
	ctx.Config.AddOrganization(orgName)
	for _, repoName := range repoNames {
		ctx.Config.AddRepository(orgName, repoName)
	}
	saveConfig(ctx)
}

func SetVerbose(ctx *context.Context, verbose bool) {
	ctx.Config.SetVerbose(verbose)
	saveConfig(ctx)
}

func SetTaggerName(ctx *context.Context, orgName, taggerName string) {
	ctx.Config.SetTaggerName(orgName, taggerName)
	saveConfig(ctx)
}

func SetTaggerEmail(ctx *context.Context, orgName, taggerEmail string) {
	ctx.Config.SetTaggerEmail(orgName, taggerEmail)
	saveConfig(ctx)
}

func saveConfig(ctx *context.Context) {
	if err := ctx.Config.Save(); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in saving the config")
	}
}
