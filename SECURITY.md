# Security Policy

## Supported Versions

Only the latest release is actively supported with security fixes.

| Version | Supported |
| ------- | --------- |
| Latest  | ✅        |
| Older   | ❌        |

## Reporting a Vulnerability

**Please do not open a public GitHub issue for security vulnerabilities.**

Report security issues privately by emailing: **pradeep.devlabs@gmail.com**

Include:
- A description of the vulnerability and its potential impact
- Steps to reproduce (proof-of-concept if possible)
- Your suggested fix or mitigation (optional)

You can expect an acknowledgement within **72 hours** and a resolution timeline within **14 days** for confirmed issues.

## Token & Credential Safety

`sgh-cli` stores per-org tokens in your OS keyring (macOS Keychain, Windows Credential Manager, Linux Secret Service) — the config file (`~/sgh.json` on Windows, `~/.config/sgh/sgh.json` on Linux/macOS) records only that a token exists and where, never its value. If no keyring is available (e.g. some headless Linux/container setups), `sgh` falls back to storing the token in the config file in plain text, with a warning. Take these precautions:

- **Never commit the config file to version control** — add it to `.gitignore`, regardless of which storage mode is in use
- The config file is written with `0600` permissions (owner read/write only) on Unix systems
- Prefer fine-grained PATs scoped to specific repositories over classic PATs
- Rotate tokens immediately if you suspect exposure

## Scope

The following are **in scope** for vulnerability reports:

- Token leakage or insecure storage
- Privilege escalation via crafted config files
- Command injection via user-supplied arguments
- Dependency vulnerabilities with direct exploitable impact

The following are **out of scope**:

- GitHub API rate-limit abuse (mitigated by the built-in rate limiter)
- Vulnerabilities in the GitHub API itself (report those to GitHub)
- Issues requiring physical access to the user's machine
