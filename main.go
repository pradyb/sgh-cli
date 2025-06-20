/*
Copyright © 2024 Pradeep Kumar Balakrishnan <pradeep.dev@proton.me>
*/
package main

import (
	"fmt"
	"os"

	"github.com/prady-lab/sgh-cli/cmd"
	"github.com/prady-lab/sgh-cli/pkg/context"
	"github.com/prady-lab/sgh-cli/pkg/logger"
)

func main() {
	ctx, err := context.Init()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing context: %v\n", err)
		logger.Glog.Error().Err(err).Msg("Error in initializing the context")
		os.Exit(1)
	}

	rootCmd := cmd.NewRootCommand(ctx)
	if err := rootCmd.Execute(); err != nil {
		logger.Glog.Error().Err(err).Msg("Command execution failed")
		os.Exit(1)
	}
}
