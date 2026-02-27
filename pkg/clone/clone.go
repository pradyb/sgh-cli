package clone

import (
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
	"github.com/prady-lab/sgh-cli/pkg/repo"
	"github.com/prady-lab/sgh-cli/pkg/ui"
)

func CloneRepositories(ctx *context.Context, orgName string, repos []string, branch string) error {
	repositories, err := repo.GetReposForOrg(ctx, orgName, false)
	if err != nil {
		return err
	}

	repoNames := make([]string, 0)

	if len(repos) == 0 {
		logger.Flog.Info().Msgf("%s for all configured repositories in %s", "Cloning", orgName)
		orgRepoNames, err := repo.GetSelectedRepoNames(ctx, orgName)
		if err != nil {
			logger.Glog.Error().Err(err).Msgf("Error in getting the Repos for the organization %s", orgName)
			return err
		}
		repoNames = append(repoNames, orgRepoNames...)
		ui.PrintSelectedRepos("Cloning", orgName, nil)
	} else {
		actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, repos)
		logger.Flog.Info().Str("repos", strings.Join(actualRepoNames, ",")).Msgf("Cloning for selected repositories in %s", orgName)
		repoNames = append(repoNames, actualRepoNames...)
	}

	if len(repoNames) != 0 {
		for _, repo := range repositories {
			if slices.Contains(repoNames, repo.Name) {
				if err := executeCloneCmd(repo, branch); err != nil {
					logger.Glog.Error().Err(err).Msgf("Error in cloning the repository %s", repo.Name)
				}
			}
		}
	} else {
		logger.Flog.Warn().Msgf("No repositories selected for cloning")
	}
	return nil
}

func executeCloneCmd(repo model.Repository, branch string) error {
	var cloneCmd *exec.Cmd
	logger.Flog.Info().Msgf("Cloning the repository %s: %s", repo.Name, repo.SSHUrl)
	if branch == "" {
		cloneCmd = exec.Command("git", "clone", repo.SSHUrl)
	} else {
		cloneCmd = exec.Command("git", "clone", "-b ", branch, repo.SSHUrl)
	}
	cloneCmd.Stdout = os.Stdout
	if err := cloneCmd.Run(); err != nil {
		return err
	}
	return nil
}
