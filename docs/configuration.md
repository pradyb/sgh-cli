# Configuration

- [Config file location](#config-file-location)
- [Managing configuration](#managing-configuration)
- [Per-owner tokens](#per-owner-tokens)
- [Owner type auto-detection](#owner-type-auto-detection)
- [Sample configuration file](#sample-configuration-file)
- [Repository include / exclude filtering](#repository-include--exclude-filtering)
- [`repositories` vs `repo_patterns`](#repositories-vs-repo_patterns)

## Config file location

| OS | Path |
|---|---|
| Windows | `~/sgh.json` |
| Linux / macOS | `~/.config/sgh/sgh.json` |

The file lives in your home directory, never inside a repository. It stores tokens in plain text, so keep it out of version control and never copy it into a project folder.

## Managing configuration

```bash
# View current configuration, including token status and owner type per org
sgh config list

# Check the file for errors — bad regex, duplicate orgs, malformed entries
sgh config validate

# Add an organization
sgh config add org my-org

# Add repositories to the fuzzy-match dictionary used by -r
sgh config add repo api-gateway --org my-org
sgh config add repo service-auth --org my-org

# Include patterns — only repos matching these are processed
sgh config add pattern "^api-" --org my-org --include
sgh config add pattern "^service-" --org my-org --include

# Exclude patterns — these repos are always skipped
sgh config add pattern ".*-legacy$" --org my-org --exclude
sgh config add pattern ".*-archive$" --org my-org --exclude

# Tagger identity, used by `tag create`
sgh config set tagger-name "Release Bot" --org my-org
sgh config set tagger-email "releases@my-org.com" --org my-org

# Remove one organization (prompts for confirmation)
sgh config reset --org my-org

# Remove everything, skipping the prompt
sgh config reset --yes
```

## Per-owner tokens

If you work across several organizations, or use a personal account alongside an org, store a dedicated fine-grained token for each owner. `sgh` picks the right one automatically based on `--org` — no manual switching.

```json
{
  "organizations": [
    { "name": "my-org",       "token": "github_pat_orgtoken..." },
    { "name": "your-username", "token": "github_pat_personaltoken..." }
  ]
}
```

Set one from the command line:

```bash
sgh config set token github_pat_xxx --org my-org
```

The `token` field is optional. Omit it for any owner that should fall back to the `SGH_TOKEN` environment variable. See [token resolution order](authentication.md#token-resolution-order).

## Owner type auto-detection

The GitHub API uses different endpoints for organizations and personal accounts. The first time you run a command against an owner, `sgh` detects which it is and caches the answer in `owner_type`, so later runs skip the lookup and spend zero extra API calls.

You can also set it ahead of time to avoid that first-run detection:

```bash
sgh config set owner-type User --org your-username
sgh config set owner-type Organization --org my-org
```

## Sample configuration file

```json
{
  "no_of_workers": 5,
  "organizations": [
    {
      "name": "my-org",
      "token": "github_pat_xxx",
      "repositories": ["api-gateway", "service-auth", "service-billing"],
      "repo_patterns": {
        "include": ["^api-", "^service-"],
        "exclude": ["legacy-.*", ".*-deprecated$", ".*-archive$"]
      },
      "pull_request_assignees": ["jane-doe", "john-smith"],
      "tagger": {
        "name": "Release Bot",
        "email": "release@my-org.com"
      }
    },
    {
      "name": "your-username",
      "token": "github_pat_yyy",
      "owner_type": "User"
    }
  ]
}
```

> **Patterns are Go regular expressions, not globs.** Write `^api-` to match repos starting with `api-`; `api-*` does not mean what you would expect — in regex it matches `api` followed by any number of `-` characters.

## Repository include / exclude filtering

When you do not pass `-r` / `--repository`, `sgh` uses `repo_patterns` to decide which repositories to act on. Rules are evaluated in this order:

| Priority | Condition | Result |
|---|---|---|
| 1 | No `include` **and** no `exclude` patterns configured | Include **all** repos |
| 2 | Repo matches any `exclude` pattern | **Always excluded** — even if it also matches an include pattern |
| 3 | `include` patterns configured and repo matches at least one | Included |
| 3 | `include` patterns configured and repo matches none | Excluded |
| 4 | Only `exclude` patterns configured, repo does not match | Included |

**With both include and exclude:**

```
include=[^api-, ^service-]   exclude=[.*-legacy$]

  api-gateway    → included   matches include
  service-auth   → included   matches include
  api-legacy     → EXCLUDED   matches exclude — exclude always wins
  web-frontend   → EXCLUDED   include is active but nothing matched
  random-tool    → EXCLUDED   include is active but nothing matched
```

**With exclude only:**

```
include=[]   exclude=[.*-archive$, .*-deprecated$]

  api-gateway         → included   no include filter, not excluded
  old-app-archive     → EXCLUDED   matches exclude
  service-deprecated  → EXCLUDED   matches exclude
  web-frontend        → included   no include filter, not excluded
```

## `repositories` vs `repo_patterns`

These two config fields are easy to confuse. They do unrelated jobs:

| Field | Purpose |
|---|---|
| `repositories` | A fuzzy-match dictionary for the `-r` flag, so `-r api` resolves to `api-gateway`. Auto-populated as you use the tool. **Not** used for filtering when `-r` is absent. |
| `repo_patterns.include` / `.exclude` | Regex rules controlling which repos are selected when **no** `-r` flag is given. Exclude always wins. |

> **Exclude applies even to explicit `-r`.** If a repository matches an `exclude` pattern, it is skipped even when you name it directly with `-r`. This is deliberate — it makes an exclude list a hard safety rail for repos that should never be touched in bulk.

---

**Next:** [Commands](commands.md) · [Advanced usage](advanced.md)
