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
)

func CloneRepositories(ctx *context.Context, orgName string, repos []string, branch string) error {
	repositories, err := repo.GetReposForOrg(ctx, orgName, false)
	if err != nil {
		return err
	}

	repoNames := make([]string, 0)
	actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(orgName, repos)
	logger.Glog.Info().Str("repos", strings.Join(actualRepoNames, ",")).Msgf("Cloning for selected repositories in %s", orgName)
	repoNames = append(repoNames, actualRepoNames...)

	if len(repoNames) != 0 {
		for _, repo := range repositories {
			if slices.Contains(repoNames, repo.Name) {
				if err := executeCloneCmd(repo, branch); err != nil {
					logger.Glog.Error().Err(err).Msgf("Error in cloning the repository %s", repo.Name)
				}
			}
		}
	} else {
		logger.Glog.Warn().Msgf("No repositories selected for cloning")
	}
	return nil
}

func executeCloneCmd(repo model.Repository, branch string) error {
	var cloneCmd *exec.Cmd
	logger.Glog.Info().Msgf("Cloning the repository %s: %s", repo.Name, repo.SSHUrl)
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
