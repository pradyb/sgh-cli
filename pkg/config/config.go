package config

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/ui"
)

var (
	successStyle = lipgloss.NewStyle().Foreground(ui.Green)
	warnStyle    = lipgloss.NewStyle().Foreground(ui.Yellow)
)

func AddOrganization(ctx *context.Context, orgName string) bool {
	if !ctx.Config.AddOrganization(orgName) {
		fmt.Println(warnStyle.Render(fmt.Sprintf("  Organization %s already present", orgName)))
		return true
	}
	saveConfig(ctx)
	fmt.Println(successStyle.Render(fmt.Sprintf("  Organization %s added successfully", orgName)))
	return false
}

func AddRepository(ctx *context.Context, orgName string, repoName string) bool {
	if !ctx.Config.AddRepository(orgName, repoName) {
		fmt.Println(warnStyle.Render(fmt.Sprintf("  Repository %s already present", repoName)))
		return true
	}
	saveConfig(ctx)
	fmt.Println(successStyle.Render(fmt.Sprintf("  Repository %s added successfully", repoName)))
	return false
}

func AddPullRequestAssignee(ctx *context.Context, orgName string, assignee string) bool {
	if !ctx.Config.AddPullRequestAssignee(orgName, assignee) {
		fmt.Println(warnStyle.Render(fmt.Sprintf("  Pull request assignee %s already present", assignee)))
		return true
	}
	saveConfig(ctx)
	fmt.Println(successStyle.Render(fmt.Sprintf("  Pull request assignee %s added successfully", assignee)))
	return false
}

func AddRepositoryPattern(ctx *context.Context, orgName string, include bool, exclude bool, pattern string) bool {
	if !ctx.Config.AddRepositoryPattern(orgName, include, exclude, pattern) {
		fmt.Println(warnStyle.Render("  Repository pattern already present"))
		return true
	}
	saveConfig(ctx)
	fmt.Println(successStyle.Render(fmt.Sprintf("  Repository pattern %s added successfully", pattern)))
	return false
}

func SaveRepositoryNamesForFuzzySearch(ctx *context.Context, orgName string, repoNames []string) {
	ctx.Config.AddOrganization(orgName)
	for _, repoName := range repoNames {
		ctx.Config.AddRepository(orgName, repoName)
	}
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
