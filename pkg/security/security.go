// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package security

import (
	"fmt"
	"strings"

	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/processor"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
)

type AlertListRequest struct {
	OrgName          string
	RepoNames        []string
	ExcludeRepoNames []string
	State            string
	SecretType       string
}

type AlertViewRequest struct {
	OrgName     string
	RepoName    string
	AlertNumber int
}

type AlertUpdateRequest struct {
	OrgName           string
	RepoName          string
	AlertNumber       int
	State             string
	Resolution        string
	ResolutionComment string
}

func ListSecretScanningAlerts(ctx *context.Context, req AlertListRequest) []model.SecretScanningAlert {
	responses := make([]model.SecretScanningAlert, 0)

	processor.ProcessRepositoriesOperation(ctx, req.OrgName, req.RepoNames, req.ExcludeRepoNames, processor.OperationListSecretScanningAlerts,
		func(ctx *context.Context, orgName, repoName string) ([]model.SecretScanningAlert, error) {
			alerts, err := service.ListSecretScanningAlerts(ctx, orgName, repoName, req.State)
			if err != nil {
				return nil, err
			}
			if req.SecretType != "" {
				filtered := make([]model.SecretScanningAlert, 0, len(alerts))
				for _, a := range alerts {
					if strings.EqualFold(a.SecretType, req.SecretType) {
						filtered = append(filtered, a)
					}
				}
				return filtered, nil
			}
			return alerts, nil
		},
		func(repoName string, result processor.RepoOperationResult[[]model.SecretScanningAlert]) {
			responses = append(responses, result.Result...)
		},
		func(repoName string, err error) {
			responses = append(responses, model.SecretScanningAlert{
				RepositoryName: repoName,
				ErrorMessage:   fmt.Sprintf("failed to list secret scanning alerts: %v", err),
			})
		})

	return responses
}

func GetSecretScanningAlert(ctx *context.Context, req AlertViewRequest) model.SecretScanningAlert {
	actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(req.OrgName, []string{req.RepoName})
	if len(actualRepoNames) == 0 {
		return model.SecretScanningAlert{ErrorMessage: fmt.Sprintf("repository not found: %s", req.RepoName)}
	}
	repoName := actualRepoNames[0]

	alert, err := service.GetSecretScanningAlert(ctx, req.OrgName, repoName, req.AlertNumber)
	if err != nil {
		return model.SecretScanningAlert{
			RepositoryName: repoName,
			ErrorMessage:   fmt.Sprintf("failed to get secret scanning alert: %v", err),
		}
	}
	return alert
}

func UpdateSecretScanningAlert(ctx *context.Context, req AlertUpdateRequest) model.SecretScanningAlert {
	actualRepoNames := ctx.Config.ActualRepositoryNamesUsingFzf(req.OrgName, []string{req.RepoName})
	if len(actualRepoNames) == 0 {
		return model.SecretScanningAlert{ErrorMessage: fmt.Sprintf("repository not found: %s", req.RepoName)}
	}
	repoName := actualRepoNames[0]

	alert, err := service.UpdateSecretScanningAlert(ctx, req.OrgName, repoName, req.AlertNumber, req.State, req.Resolution, req.ResolutionComment)
	if err != nil {
		return model.SecretScanningAlert{
			RepositoryName: repoName,
			ErrorMessage:   fmt.Sprintf("failed to update secret scanning alert: %v", err),
		}
	}
	return alert
}
