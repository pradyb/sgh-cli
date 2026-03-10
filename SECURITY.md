# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest  | Yes       |

## Reporting a Vulnerability

**Please do not report security vulnerabilities through public GitHub issues.**

If you discover a security vulnerability in sgh-cli, please report it responsibly:

1. **Email:** Send details to `pradeep.dev@proton.me` with the subject line `[SECURITY] sgh-cli vulnerability`
2. **GitHub Private Advisory:** Use [GitHub's private security advisory](https://github.com/pradyb/sgh-cli/security/advisories/new) feature

### What to Include

Please provide as much of the following information as possible:

- Type of vulnerability (e.g., credential exposure, command injection, path traversal)
- Affected component and version
- Step-by-step reproduction instructions
- Proof-of-concept or exploit code (if available)
- Potential impact assessment

### Response Timeline

- **Acknowledgement:** Within 48 hours
- **Initial assessment:** Within 5 business days
- **Fix + coordinated disclosure:** Within 90 days (depending on severity)

## Security Considerations

### GitHub Token Handling

sgh-cli **never** stores your GitHub token to disk. The token is read exclusively from the `GITHUB_TOKEN` environment variable at runtime. We validate token format on startup to detect accidental misuse.

### Token Permissions

Follow the principle of least privilege. Grant only the scopes your workflow actually needs:

| Operation | Required Scopes |
|-----------|----------------|
| Read repos/PRs/branches | `repo:read` |
| Create/update branches, PRs | `repo` |
| Team management | `admin:org` |
| Secret scanning alerts | `security_events` |

### Network Security

All API calls are made over HTTPS to `api.github.com`. sgh-cli does not make calls to any third-party services.

### Dependency Security

Dependencies are pinned in `go.sum`. Run the following to audit dependencies:

```bash
go list -m all | nancy sleuth
# or
govulncheck ./...
```
