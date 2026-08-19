// Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.devlabs@gmail.com>
// SPDX-License-Identifier: MIT

package service

import (
	"context"

	appcontext "github.com/pradyb/sgh-cli/pkg/context"
)

func Query(ctx *appcontext.Context, query interface{}, variables map[string]interface{}) error {
	return QueryWithContext(context.Background(), ctx, query, variables)
}

func QueryWithContext(reqCtx context.Context, ctx *appcontext.Context, query interface{}, variables map[string]interface{}) error {
	return ctx.GraphqlClient.QueryWithContext(reqCtx, query, variables)
}
