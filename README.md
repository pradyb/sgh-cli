# Simple Github CLI

### Commands to perform Github operations.

```sh

Usage:
  sgh <command> <subcommand> [flags]
  sgh [command]

Examples:
$ sgh branch create
$ sgh tag create
$ sgh pb update --org sample-org -r sample-repo1 --branch sample-branch  -l -d
$ sgh pr list --org sample-org -r sample-repo1 -r sample-repo2 --base "develop"
$ sgh post-release -o sample-org -r sample-repo1 -r sample-repo2 --base "main" --head "Release-1.0" --create-tag


Available Commands:
  branch       Manage branches
  clone        Clone all the selected repositories for the given owner/organization
  commit       Manage commits
  config       Manage configuration for sgh
  help         Help about any command
  pb           Manage protected branches
  post-release Perform Post release activities like merging to main/develop and tagging
  pr           Manage pull requests
  repo         Repository operations for the given organization
  tag          Manage tags.
  team         Organization teams

Flags:
  -h, --help           help for sgh
  -L, --log-response   log HTTP response
  -o, --org string     organization name
  -v, --verbose        verbose output
  -w, --workers int    number of workers (default 5)

Use "sgh [command] --help" for more information about a command.

```