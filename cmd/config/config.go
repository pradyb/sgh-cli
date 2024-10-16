package config

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/prady-lab/sgh-cli/pkg/config"
	"github.com/prady-lab/sgh-cli/pkg/context"

	"github.com/prady-lab/sgh-cli/pkg/logger"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
)

func NewConfigCommand(ctx *context.Context) *cobra.Command {

	var configCmd = &cobra.Command{
		Use:   "config <command>",
		Short: "Manage configuration for sgh",
		Long:  `Add/Remove/List the configuration for sgh.`,
		Example: heredoc.Doc(`
			$ sgh config list
			$ sgh config add key value
		`),
	}

	configCmd.AddCommand(addCommand(ctx))
	configCmd.AddCommand(setCommand(ctx))

	return configCmd
}

var include bool
var exclude bool

func addCommand(ctx *context.Context) *cobra.Command {
	var configAddCmd = &cobra.Command{
		Use:   "add <key> <value>",
		Short: "Add a configuration for sgh",
		Long: `Add following configurations for sgh:
New organization
New repository to organization 
include/exclude patterns to select the repository.`,
		Example: heredoc.Doc(`
			$ sgh config add org sample-org
			$ sgh config add repo sample-repo -o sample-org
			$ sgh config add pattern abc-* -o sample-org -i
			$ sgh config add pattern xyz-* -o sample-org -e
		`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("invalid arguments")
			}
			key := args[0]
			orgName, _ := cmd.Flags().GetString("org")
			if slices.Contains([]string{"repo", "repository", "pattern", "pr-assignee"}, strings.ToLower(key)) && orgName == "" {
				return fmt.Errorf("organization name is required")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			value := args[1]
			orgName, _ := cmd.Flags().GetString("org")

			switch strings.ToLower(key) {
			case "org", "organization":
				config.AddOrganization(ctx, value)
				return
			case "repo", "repository":
				config.AddRepository(ctx, orgName, value)
				return
			case "pattern":
				if include && exclude {
					logger.Glog.Error().Msgf("Both include and exclude can't be true")
					return
				}
				_, err := regexp.Compile(value)
				if err != nil {
					logger.Glog.Error().Msgf("Invalid pattern %s", value)
					return
				}
				config.AddRepositoryPattern(ctx, orgName, include, exclude, value)
				return
			case "pr-assignee":
				config.AddPullRequestAssignee(ctx, orgName, value)
				return
			default:
				logger.Glog.Error().Msgf("Invalid Key %s", key)
			}
		},
	}

	configAddCmd.Flags().BoolVarP(&include, "include", "i", false, "The `regex pattern` to select the repositories to include for processing")
	configAddCmd.Flags().BoolVarP(&exclude, "exclude", "e", false, "The `regex pattern` if you want to exclude some repositories from processing")
	configAddCmd.MarkFlagsMutuallyExclusive("include", "exclude")
	return configAddCmd
}

func setCommand(ctx *context.Context) *cobra.Command {
	var configSetCmd = &cobra.Command{
		Use:   "set",
		Short: "Set a configuration for sgh",
		Long:  `Set attribute values`,
		Example: heredoc.Doc(`
			$ sgh config set tagger-name "John Doe"
			$ sgh config set tagger-email "john.doe@sample.com"
		`),
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return fmt.Errorf("invalid arguments")
			}
			key := args[0]
			orgName, _ := cmd.Flags().GetString("org")
			if slices.Contains([]string{"tagger-name", "tagger-email"}, strings.ToLower(key)) && orgName == "" {
				return fmt.Errorf("organization name is required")
			}
			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			key := args[0]
			value := args[1]
			orgName, _ := cmd.Flags().GetString("org")

			switch strings.ToLower(key) {
			case "tagger-name":
				config.SetTaggerName(ctx, orgName, value)
				return
			case "tagger-email":
				config.SetTaggerEmail(ctx, orgName, value)
				return
			default:
				logger.Glog.Error().Msgf("Invalid Key %s", key)
			}
		},
	}
	return configSetCmd
}
