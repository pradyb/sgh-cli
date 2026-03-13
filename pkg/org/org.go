// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

// Package org provides functions for fetching GitHub organization details.
package org

import (
	"github.com/prady-lab/sgh-cli/internal/model"
	"github.com/prady-lab/sgh-cli/internal/service"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
)

// ListOrgs returns details for every organization the authenticated token belongs to.
func ListOrgs(ctx *context.Context) []model.OrgDetail {
	results, err := service.ListOrganizations(ctx)
	if err != nil {
		logger.Glog.Error().Err(err).Msg("failed to fetch organizations")
		ctx.HasError = true
		return nil
	}
	return results
}
