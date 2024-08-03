/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"os"

	"github.com/prady-lab/sgh-cli/cmd"
	"github.com/prady-lab/sgh-cli/pkg/context"
	logger "github.com/prady-lab/sgh-cli/utils"
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
