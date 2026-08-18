# Security Policy

## Supported Versions

| Version | Supported          |
|---------|--------------------|
| 1.x     | :white_check_mark: |

## Reporting a Vulnerability

We take security issues seriously. Please **do not** open a public issue for
security vulnerabilities.

To report a vulnerability, please contact the maintainers privately (e.g. via
GitHub Security Advisory "Report a vulnerability" on this repository, or the
maintainer email listed on the GitHub profile).

Please include:

- Affected version(s)
- A description of the vulnerability and its impact
- Steps to reproduce (if possible)

We will acknowledge receipt within 5 business days and work with you to
remediate the issue before disclosure.

## Best practices for users

- Grant the database account used by Uptime-Monitor the **least privileges**
  needed (e.g. `CONNECT` session only for a `SELECT 1 FROM DUAL` check).
- Prefer injecting credentials via environment variables (`UM_DB_PASSWORD`)
  or secrets manager over plain-text files.
- Restrict network access to the monitoring host and the Uptime Kuma instance.
