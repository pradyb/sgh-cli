# sgh-cli documentation

Full documentation for [sgh-cli](../README.md). Start with installation and authentication; everything else is reference.

## Setup

| Page | What's in it |
|---|---|
| [Installation](installation.md) | Go install, prebuilt binaries with checksums, building from source, shell completion |
| [Authentication](authentication.md) | Fine-grained vs classic PATs, the exact permissions each feature needs, token resolution order |
| [Configuration](configuration.md) | Config file format, per-owner tokens, include/exclude filtering rules |

## Reference

| Page | What's in it |
|---|---|
| [Command reference](commands.md) | Every command, alias, and subcommand; global flags; the shorthand convention |
| [Usage examples](examples.md) | Working examples for each command group |
| [Advanced usage](advanced.md) | Repository targeting, JSON/compact output for scripting, concurrency, rate limits |

## Help and contributing

| Page | What's in it |
|---|---|
| [Troubleshooting](troubleshooting.md) | Token rejections, 403s, rate limits, empty result sets |
| [Development](development.md) | Building, testing, coverage, project layout, architecture |
| [Contributing](../CONTRIBUTING.md) | Pull request process |
| [Security policy](../SECURITY.md) | Reporting a vulnerability |

## Quick answers

- **Which permissions does my token need?** → [Repository permissions](authentication.md#repository-permissions)
- **Why is nothing being processed?** → [No repositories are processed](troubleshooting.md#no-repositories-are-processed)
- **How do I use this in a script?** → [Output modes](advanced.md#output-modes-and-scripting)
- **How do I work across several orgs?** → [Per-owner tokens](configuration.md#per-owner-tokens)
- **What does `--limit` do that `--last` doesn't?** → [Global flags](commands.md#global-flags)
