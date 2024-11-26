# Simple Github CLI

### Commands to perform Github operations.

```sh

Simple CLI to process the all or selected repositories in an organization.

Usage:
  sgh <command> <subcommand> [flags]
  sgh [command]

Examples:
$ sgh branch create --org sample-org --new Release-1.1 --ref Release-1.0
$ sgh tag create --org sample-org --tag Release-1.0 --head Release-1.0 --message 'Tag for Release 1.0'
$ sgh pb update --org sample-org --branch sample-branch --repo sample-repo1 -l -d --add-bypass-user john-doe --add-push-user jane-doe
$ sgh pr list --org sample-org --repo sample-repo1 --repo sample-repo2 --base "develop"
$ sgh post-release --org sample-org --base "main" --head "Release-1.0" --create-tag --title "Release 1.0"


Available Commands:
  branch       Create and delete branches
  clone        Clone all the selected repositories for the given owner/organization
  commit       List recent commits for all the repositories
  config       Manage configuration for sgh
  help         Help about any command
  pb           Perform operations like view/update/delete protected branches.
  post-release Perform Post release activities like merging to main/develop and tagging
  pr           Perform PR operations like create/review/merge/close/update/list
  repo         List the repositories details for the given owner/organization
  tag          Create and delete tags
  team         List teams and corresponding members in each team

Flags:
  -h, --help           help for sgh
  -L, --log-response   log HTTP response
  -o, --org string     organization name
  -v, --verbose        verbose output
  -w, --workers int    number of workers (default 5)

Use "sgh [command] --help" for more information about a command.

```