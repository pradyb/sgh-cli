# Troubleshooting

Start here for anything that fails before you open an issue:

```bash
sgh health          # connectivity, token validity, rate limit status
sgh whoami          # which identity the token actually resolves to
sgh config validate # config file errors — bad regex, duplicate orgs
```

- [Token is rejected](#token-is-rejected)
- [403 Forbidden / permission errors](#403-forbidden--permission-errors)
- [Rate limiting](#rate-limiting)
- [No repositories are processed](#no-repositories-are-processed)
- [Network and API errors](#network-and-api-errors)
- [Configuration problems](#configuration-problems)
- [Command not found](#command-not-found)

## Token is rejected

`sgh` validates the token's shape before any network call, so a format error fails immediately:

- At least 20 characters
- Starts with `ghp_`, `gho_`, `ghu_`, `ghs_`, `ghr_`, or `github_pat_`
- No spaces — a common cause is a trailing newline or a quoted value that captured one
- Not a test token (`ghp_test_...` is rejected deliberately)

If the format is fine but requests still fail, the token is likely expired or revoked. `sgh whoami` confirms whether it resolves to an identity at all.

## 403 Forbidden / permission errors

In order of likelihood:

1. **Missing permission.** Fine-grained PATs grant nothing by default. Check your token against the [permissions table](authentication.md#repository-permissions) for the specific feature — `Administration` for protected branches, `Secret scanning alerts` for `security`, `Organization Administration` for `audit`.
2. **SAML SSO not authorized.** If the organization enforces SSO, you must explicitly authorize the token for that org after creating it. Until you do, every request 403s regardless of scopes.
3. **Fine-grained PAT scoped to the wrong owner.** A token issued for your personal account cannot see organization repositories. Check with `sgh whoami` and, if you work across owners, use [per-owner tokens](configuration.md#per-owner-tokens).
4. **Insufficient repository role.** Branch protection and visibility changes need admin on the repository, not just write.

## Rate limiting

```bash
curl -H "Authorization: token $SGH_TOKEN" https://api.github.com/rate_limit
```

`sgh` retries with exponential backoff and trips a circuit breaker rather than hammering a limited API, so sustained 403s usually mean you have genuinely exhausted the hourly budget. Reduce concurrency and narrow the target set:

```bash
sgh pr create --org my-org --workers 2 --title "Update"
```

See [Rate limits](advanced.md#rate-limits) for the full guidance.

## No repositories are processed

Almost always a filtering surprise rather than a bug. Check what your config actually says:

```bash
sgh config list
```

Common causes:

- **An `include` pattern is set and nothing matches it.** Once any include pattern exists, repositories that match none of them are excluded.
- **Glob syntax where regex is expected.** Patterns are Go regular expressions. `api-*` does not mean "starts with api-"; write `^api-`.
- **An `exclude` pattern is catching more than intended.** Exclude always wins, even over an explicit `-r`.

The precedence rules are tabulated in [Configuration](configuration.md#repository-include--exclude-filtering).

## Network and API errors

```bash
# Full request/response logging
sgh repo list --org my-org --log-response --verbose

# Raise the HTTP timeout for slow networks or very large orgs
SGH_TIMEOUT=60s sgh repo list --org my-org
```

Behind a corporate proxy, `sgh` honours the standard `HTTP_PROXY`, `HTTPS_PROXY`, and `NO_PROXY` environment variables:

```bash
export HTTPS_PROXY=http://proxy.example.com:8080
export NO_PROXY=localhost,127.0.0.1
```

If your proxy terminates TLS with a private CA, that CA must be trusted by the system store — `sgh` does not accept a custom CA bundle flag.

## Configuration problems

```bash
sgh config list       # what is currently loaded
sgh config validate   # regex errors, duplicate orgs, malformed entries

# Remove one organization, or start over
sgh config reset --org my-org
sgh config reset --yes

# Or delete the file directly
rm ~/.config/sgh/sgh.json   # Linux/macOS
rm ~/sgh.json               # Windows
```

## Command not found

```bash
which sgh   # Linux/macOS
where sgh   # Windows
```

If you installed with `go install`, the binary is named `sgh-cli` and lives in `$(go env GOPATH)/bin` — which may not be on your `PATH`. See [Installation](installation.md#option-1-go-install).

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

---

Still stuck? Open an issue with the output of `sgh version` and `sgh health`, plus the failing command run with `--verbose`. Redact your token.
