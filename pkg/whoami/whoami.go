// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
// SPDX-License-Identifier: MIT

package whoami

import (
	"github.com/pradyb/sgh-cli/internal/model"
	"github.com/pradyb/sgh-cli/internal/service"
	"github.com/pradyb/sgh-cli/pkg/context"
	"github.com/pradyb/sgh-cli/pkg/logger"
)

// GetCurrentUser returns the authenticated user's profile.
func GetCurrentUser(ctx *context.Context) *model.UserInfo {
	user, err := service.GetCurrentUser(ctx)
	if err != nil {
		logger.Glog.Error().Err(err).Msg("failed to fetch user info")
		ctx.HasError = true
		return nil
	}
	return user
}
