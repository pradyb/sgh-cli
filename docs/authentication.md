# Authentication

`sgh` authenticates to GitHub with a Personal Access Token. Nothing else is supported — there is no OAuth flow and no GitHub App installation.

- [Fine-grained PAT (recommended)](#fine-grained-pat-recommended)
- [Classic PAT](#classic-pat)
- [Providing the token](#providing-the-token)
- [Token resolution order](#token-resolution-order)
- [Token requirements](#token-requirements)

## Fine-grained PAT (recommended)

Fine-grained PATs give least-privilege access and work for both personal accounts and organizations.

1. Go to **Settings → Developer settings → Personal access tokens → Fine-grained tokens**
2. Click **Generate new token**
3. Under **Resource owner**, select the organization or your personal account
4. Under **Repository access**, choose **All repositories** or select specific ones
5. Grant the permissions below, based on which features you actually need

### Repository permissions

| Permission | Access | Enables |
|---|---|---|
| **Contents** | Read & Write | Branch/tag create & delete, clone, commit list |
| **Metadata** | Read-only | Auto-required — repo listing and search |
| **Pull requests** | Read & Write | PR list, create, merge, approve, review, assignees |
| **Issues** | Read & Write | Issue list, view, create, assignees |
| **Actions** | Read & Write | Workflow run list, view, rerun, cancel, dispatch |
| **Secret scanning alerts** | Read & Write | `security list`, `view`, `update` |
| **Administration** | Read & Write | Protected branch config, repo archive/visibility |
| **Commit statuses** | Read-only | Commit check-runs (used by `post-release`) |

### Organization permissions

Only needed when you point `sgh` at an organization rather than a personal account:

| Permission | Access | Enables |
|---|---|---|
| **Organization Members** | Read-only | `team list` |
| **Organization Administration** | Read-only | `audit list` |

> **Read-only minimum:** Contents (Read), Metadata (Read), Pull requests (Read), Issues (Read), Actions (Read), Secret scanning alerts (Read). This covers every `list` and `view` command while making it impossible for the tool to change anything.

## Classic PAT

Classic PATs cover all your organizations and your personal account with a single token — simpler to set up, but far broader in scope.

1. Go to **Settings → Developer settings → Personal access tokens → Tokens (classic)**
2. Click **Generate new token**
3. Select scopes: `repo`, `admin:org`, `read:audit_log`

> **Note:** GitHub is steering users toward fine-grained PATs. Classic PATs still work but may be deprecated in future.

If your organization uses SAML SSO, you must authorize the token for that organization after creating it, or every request will fail with a 403.

## Providing the token

**Linux/macOS:**

```bash
export SGH_TOKEN=your_token_here

# Persist it
echo 'export SGH_TOKEN=your_token_here' >> ~/.bashrc
```

**Windows (PowerShell):**

```powershell
$env:SGH_TOKEN = "your_token_here"

# Persist it by adding the line above to your PowerShell profile
```

**Windows (Command Prompt):**

```cmd
set SGH_TOKEN=your_token_here
```

For working across several organizations with a different token for each, store them per-owner in the config file instead — see [Per-owner tokens](configuration.md#per-owner-tokens).

## Token resolution order

For any given command, `sgh` looks for a token in this order and uses the first one it finds:

1. The `token` field in your config file for the owner named by `--org`
2. The `SGH_TOKEN` environment variable
3. The `GITHUB_TOKEN` environment variable — deprecated fallback, kept for compatibility

## Token requirements

`sgh` validates the token's shape before making any network call, so typos fail fast rather than as a confusing 401:

- At least 20 characters long
- Starts with `ghp_`, `gho_`, `ghu_`, `ghs_`, `ghr_`, or `github_pat_`
- Contains no spaces
- Is not a test token (anything starting with `ghp_test_` is rejected)

To confirm which identity a token actually resolves to:

```bash
sgh whoami
```

---

**Next:** [Configuration](configuration.md) · [Commands](commands.md)
