package service

import (
	"github.com/prady-lab/sgh-cli/pkg/context"
)

func Query(ctx *context.Context, query interface{}, variables map[string]interface{}) error {
	return ctx.GraphqlClient.Query(query, variables)
}
