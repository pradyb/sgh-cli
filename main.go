/*
Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
*/
package main

import (
	"os"

	"github.com/prady-lab/sgh-cli/cmd"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
)

func main() {
	ctx, err := context.Init()
	if err != nil {
		logger.Glog.Error().Err(err).Msg("Error in initializing the context")
		return
	}

	rootCmd := cmd.NewRootCommand(ctx)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
