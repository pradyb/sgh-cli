// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package config

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	internalconfig "github.com/prady-lab/sgh-cli/internal/config"
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

func SetToken(ctx *context.Context, orgName, token string) {
	ctx.Config.SetToken(orgName, token)
	saveConfig(ctx)
}

func SetOwnerType(ctx *context.Context, orgName, ownerType string) {
	ctx.Config.SetOwnerType(orgName, ownerType)
	saveConfig(ctx)
}

func SetTaggerName(ctx *context.Context, orgName, taggerName string) {
	ctx.Config.SetTaggerName(orgName, taggerName)
	saveConfig(ctx)
	fmt.Println(successStyle.Render(fmt.Sprintf("  Tagger name for %s set to %q", orgName, taggerName)))
}

func SetTaggerEmail(ctx *context.Context, orgName, taggerEmail string) {
	ctx.Config.SetTaggerEmail(orgName, taggerEmail)
	saveConfig(ctx)
	fmt.Println(successStyle.Render(fmt.Sprintf("  Tagger email for %s set to %q", orgName, taggerEmail)))
}

func RemoveOrganization(ctx *context.Context, orgName string) {
	if !ctx.Config.RemoveOrganization(orgName) {
		fmt.Println(warnStyle.Render(fmt.Sprintf("  Organization %q not found in config", orgName)))
		return
	}
	saveConfig(ctx)
	fmt.Println(successStyle.Render(fmt.Sprintf("  Organization %q removed", orgName)))
}

func RemoveRepository(ctx *context.Context, orgName, repoName string) {
	if !ctx.Config.RemoveRepository(orgName, repoName) {
		fmt.Println(warnStyle.Render(fmt.Sprintf("  Repository %q not found in org %q", repoName, orgName)))
		return
	}
	saveConfig(ctx)
	fmt.Println(successStyle.Render(fmt.Sprintf("  Repository %q removed from %q", repoName, orgName)))
}

func RemovePullRequestAssignee(ctx *context.Context, orgName, assignee string) {
	if !ctx.Config.RemovePullRequestAssignee(orgName, assignee) {
		fmt.Println(warnStyle.Render(fmt.Sprintf("  Assignee %q not found in org %q", assignee, orgName)))
		return
	}
	saveConfig(ctx)
	fmt.Println(successStyle.Render(fmt.Sprintf("  Assignee %q removed from %q", assignee, orgName)))
}

func RemoveRepositoryPattern(ctx *context.Context, orgName string, include bool, pattern string) {
	if !ctx.Config.RemoveRepositoryPattern(orgName, include, pattern) {
		kind := "exclude"
		if include {
			kind = "include"
		}
		fmt.Println(warnStyle.Render(fmt.Sprintf("  %s pattern %q not found in org %q", kind, pattern, orgName)))
		return
	}
	saveConfig(ctx)
	kind := "exclude"
	if include {
		kind = "include"
	}
	fmt.Println(successStyle.Render(fmt.Sprintf("  %s pattern %q removed from %q", kind, pattern, orgName)))
}

// ConfigFilePath returns the absolute path to the active sgh config file.
func ConfigFilePath() string { return internalconfig.ConfigFilePath() }

func saveConfig(ctx *context.Context) {
	if err := ctx.Config.Save(); err != nil {
		logger.Glog.Error().Err(err).Msg("Error in saving the config")
	}
}
