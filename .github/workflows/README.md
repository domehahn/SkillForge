# GitHub Actions Workflows

This directory contains the GitHub Actions workflows configured for SkillForge.
For the authoritative triggers, permissions, and required checks, see each
workflow file; this page is a navigation overview.

## Workflows Overview

### Core CI/CD

- **[skill-registry.yml](skill-registry.yml)** - Main backend workflow
  - Go testing with race detector
  - Code linting and formatting checks
  - Vulnerability scanning (govulncheck)
  - Static analysis (staticcheck)
  - Docker image build and smoke testing
  - Runs on: See the `on` declaration in the workflow file

- **[frontend.yml](frontend.yml)** - Web UI workflow
  - Build React/Vite application
  - Dependency checks
  - Bundle size analysis
  - Type checking (if configured)
  - Runs on: See the `on` declaration in the workflow file

- **[integration-tests.yml](integration-tests.yml)** - E2E testing
  - Builds complete stack (backend + frontend)
  - Runs Playwright tests
  - Generates test reports
  - Runs on: See the `on` declaration in the workflow file

### Security & Quality

- **[codeql.yml](codeql.yml)** - Advanced security scanning
  - Analyzes Go and JavaScript/TypeScript code
  - Detects security vulnerabilities
  - Runs quality checks
  - Runs on: See the `on` declaration in the workflow file

- **[base-image-refresh.yml](base-image-refresh.yml)** - Container security
  - Weekly rebuild with latest base images
  - Scans for CVEs with Trivy
  - Opens GitHub issues if vulnerabilities found
  - Runs on: Weekly (Monday 06:00 UTC), manual trigger

### Release & Deployment

- **[release.yml](release.yml)** - Production releases
  - Cross-platform binary builds (Linux, macOS, Windows)
  - Multi-architecture support (amd64, arm64)
  - Docker multi-platform images
  - Image signing with cosign
  - SBOM generation
  - GitHub Release creation
  - Runs on: See the `on` declaration in the workflow file

### Validation & Documentation

- **[validate-examples.yml](validate-examples.yml)** - Example skills validation
  - Validates skill.yaml structure
  - Checks required files (SKILL.md, VERSION, CHANGELOG.md)
  - Verifies SemVer format
  - Tests packaging
  - Runs on: See the `on` declaration in the workflow file

- **[openapi.yml](openapi.yml)** - API documentation
  - Validates OpenAPI specifications
  - Generates bundled spec
  - Creates ReDoc documentation
  - Detects breaking changes in PRs
  - Runs on: See the `on` declaration in the workflow file

- **[pr-checks.yml](pr-checks.yml)** - PR validation
  - Validates PR title format
  - Checks for merge conflicts
  - Validates commit messages
  - Detects large files
  - Scans for sensitive data patterns
  - Posts PR summary comment
  - Runs on: See the `on` declaration in the workflow file

## Automated Dependency Updates

- **[dependabot.yml](../dependabot.yml)** - Dependency management
  - Go modules (weekly)
  - npm packages for web UI (weekly)
  - npm packages for testing (weekly)
  - GitHub Actions (weekly)
  - Groups minor and patch updates

## Secrets and permissions

Workflow permissions and referenced secrets are defined in the workflow files.
GitHub's automatically provided `GITHUB_TOKEN` is used where configured.
Repository owners should review permissions when changing release or publishing
workflows.

## Status Badges

Add these to your README.md:

```markdown
[![Registry CI](https://github.com/domehahn/SkillForge/actions/workflows/skill-registry.yml/badge.svg)](https://github.com/domehahn/SkillForge/actions/workflows/skill-registry.yml)
[![Frontend CI](https://github.com/domehahn/SkillForge/actions/workflows/frontend.yml/badge.svg)](https://github.com/domehahn/SkillForge/actions/workflows/frontend.yml)
[![Integration Tests](https://github.com/domehahn/SkillForge/actions/workflows/integration-tests.yml/badge.svg)](https://github.com/domehahn/SkillForge/actions/workflows/integration-tests.yml)
[![CodeQL](https://github.com/domehahn/SkillForge/actions/workflows/codeql.yml/badge.svg)](https://github.com/domehahn/SkillForge/actions/workflows/codeql.yml)
```

## Local Testing

### Test workflows locally with act

```bash
# Install act
brew install act

# Run skill-registry workflow
act -W .github/workflows/skill-registry.yml

# Run with specific event
act pull_request -W .github/workflows/pr-checks.yml
```

### Test individual steps

```bash
# Backend tests
cd skill-registry && make test

# Frontend build
cd skill-registry/web && npm ci && npm run build

# Integration tests
cd skill-registry && npx playwright test
```

## Workflow Triggers Summary

| Workflow | Push | PR | Schedule | Tags | Manual |
|----------|------|----|----------|------|--------|
| skill-registry | ✓ | ✓ | | | |
| frontend | ✓ | ✓ | | | |
| integration-tests | ✓ | ✓ | | | ✓ |
| codeql | ✓ | ✓ | Weekly | | ✓ |
| base-image-refresh | | | Weekly | | ✓ |
| release | | | | ✓ | |
| validate-examples | ✓ | ✓ | | | ✓ |
| openapi | ✓ | ✓ | | | ✓ |
| pr-checks | | ✓ | | | |

## Maintenance

### Weekly Tasks (Automated)

- Dependency updates via Dependabot
- Base image vulnerability scans
- CodeQL security scans

### Release Process

- Tag with semantic version: `git tag v1.0.0`
- Push tag: `git push origin v1.0.0`
- Release workflow builds and publishes automatically

## Troubleshooting

### Common Issues

1. **Go tests failing**: Ensure CGO is enabled and sqlite3-dev is installed
2. **Docker build failing**: Check Dockerfile path matches the workflow
3. **Frontend build failing**: Verify node_modules and package-lock.json are in sync
4. **Integration tests timing out**: Increase timeout in workflow or optimize tests

### Debug Mode

Enable debug logging by setting repository secrets:

- `ACTIONS_STEP_DEBUG`: `true`
- `ACTIONS_RUNNER_DEBUG`: `true`

## Contributing

When adding new workflows:

1. Test locally with `act` if possible
2. Start with `workflow_dispatch` trigger for testing
3. Add appropriate path filters to avoid unnecessary runs
4. Update this README with workflow description
5. Add status badge to main README
