# Security policy

## Supported versions

| Version | Supported |
| --- | --- |
| Latest 0.1.x release | Yes |
| Older releases and unreleased branches | No |

## Reporting a vulnerability

Please report suspected vulnerabilities privately through [GitHub Private Vulnerability Reporting](https://github.com/loaiza000/eject/security/advisories/new). If that link is unavailable, open a GitHub issue that asks for a private channel, without exploit details or sensitive data. There is no dedicated security email address.

Never include credentials, real cloud exports, or private infrastructure details in a report. Synthetic reproductions are preferred.

This is a volunteer-maintained project with no service-level commitment. Confirmed issues are fixed in a new patch release and recorded in the [changelog](CHANGELOG.md).

## Scope

EJECT runs locally on files you provide and on Docker commands you explicitly authorize. Its security properties and known limits (input bounds, private output permissions, tamper detection, Docker isolation) are described in [docs/security.md](docs/security.md). Container images started by `verify --runtime --allow-start` run their own code, and their behavior is outside EJECT's control.
