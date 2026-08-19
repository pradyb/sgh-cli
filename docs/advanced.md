# Advanced usage

- [Choosing which repositories to act on](#choosing-which-repositories-to-act-on)
- [Output modes and scripting](#output-modes-and-scripting)
- [Concurrency](#concurrency)
- [Environment variables](#environment-variables)
- [Rate limits](#rate-limits)
- [Performance notes](#performance-notes)

## Choosing which repositories to act on

Two complementary mechanisms decide the target set.

### Config-based — persistent and automatic

Set `repo_patterns` once and every command respects it without further flags:

```bash
sgh config add pattern "^api-" --org my-org --include
sgh config add pattern "^service-" --org my-org --include
sgh config add pattern ".*-legacy$" --org my-org --exclude
sgh config add pattern ".*-archive$" --org my-org --exclude
```

```bash
sgh branch list   --org my-org   # only api-* and service-*
sgh pr list       --org my-org   # same filtering
sgh workflow list --org my-org   # same filtering
```

The full precedence table is in [Configuration](configuration.md#repository-include--exclude-filtering).

### Flags — per-command override

```bash
# Target specific repos (fuzzy-matched against the config repository list)
sgh branch create --org my-org --new feature --ref main -r api-gateway -r service-auth

# Skip repos for this run only
sgh pr list --org my-org -e legacy-app -e test-harness
```

> `exclude` patterns still apply even when you name a repository explicitly with `-r`. A repo on the exclude list is never touched.

## Output modes and scripting

`--output table` is the default and is meant for humans; tables auto-size to your terminal width. The other two modes are for pipelines.

```bash
# compact — tab-separated, one record per line
sgh pr list --org my-org --output compact | grep open
sgh workflow list --org my-org --running -C | awk '{print $1, $4}'

# json — structured, for jq
sgh repo list --org my-org --output json | jq '.[].name'
sgh workflow list --org my-org -J | jq '.[] | select(.status == "failure")'
```

`-C` and `-J` are shorthands for `--output compact` and `--output json`.

When scripting, also set `NO_COLOR=1` (or pass `--no-color`) so ANSI escapes never reach your parser.

## Concurrency

Bulk operations run across a worker pool, 5 workers by default.

```bash
# More workers — faster, heavier on your rate limit
sgh clone --org large-org --workers 10

# Fewer workers — slower, gentler
sgh pr create --org my-org --title "Update" --head feature --base main --workers 2
```

More workers is not always faster. Beyond roughly 10 you tend to trade throughput for secondary rate-limit rejections, especially on write operations.

## Environment variables

| Variable | Description | Default |
|---|---|---|
| `SGH_TOKEN` | GitHub Personal Access Token (**required**) | — |
| `SGH_ORG` | Default organization or user | — |
| `SGH_WORKERS` | Concurrent workers | `5` |
| `SGH_TIMEOUT` | HTTP client timeout, e.g. `60s` | `30s` |
| `SGH_VERBOSE` | Verbose output (`true`/`false`) | `false` |
| `SGH_LOG_RESPONSE` | Log HTTP responses (`true`/`false`) | `false` |
| `NO_COLOR` | Disable colored output | — |

## Rate limits

Authenticated requests get 5,000 per hour against the REST API. `sgh` tracks remaining quota, backs off automatically with exponential retry, and opens a circuit breaker when GitHub starts refusing requests — but a wide enough bulk operation can still exhaust the budget.

```bash
# Current quota
curl -H "Authorization: token $SGH_TOKEN" https://api.github.com/rate_limit
```

If you are hitting limits:

- Drop `--workers` to 2–3
- Narrow the target set with `-r` or config `include` patterns
- Use `--last` to fetch fewer items per repository
- Prefer one wide command over many narrow ones — repository listings are cached per run

## Performance notes

- **Worker count** is the main lever; the default of 5 suits most organizations
- **Filter early** — excluding repositories costs nothing, fetching them costs a request each
- **`--verbose`** shows per-repository timing, which is how you find the slow one
- **Config over flags** — patterns in config apply everywhere and cannot be forgotten
- **`owner_type` caching** saves one API call per run; set it explicitly for owners you use often

---

**Next:** [Troubleshooting](troubleshooting.md) · [Development](development.md)
