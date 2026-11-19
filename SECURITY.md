# Security policy

## Reporting a vulnerability

Please report suspected vulnerabilities privately through
[GitHub's security advisory reporting for this repository](https://github.com/domehahn/SkillForge/security/advisories/new).
Include the affected component and version, steps to reproduce, and any
potential impact. Do not include credentials or private user data in the
report.

Please do not disclose an unpatched vulnerability in a public issue. The
maintainers will acknowledge reports and coordinate a fix and disclosure with
the reporter.

## Deployment note

Treat registry tokens, signing keys, database credentials, and model-provider
credentials as secrets. Do not commit them or put literal credentials in
artifact manifests. See the artifact documentation for secret references and
local runtime boundaries.
