# GitHub Actions Workflows

This directory contains all CI/CD workflows for SkillForge.

## Workflows Overview

### Core CI/CD

- **[skill-registry.yml](skill-registry.yml)** - Main backend workflow
  - Go testing with race detector
  - Code linting and formatting checks
  - Vulnerability scanning (govulncheck)
  - Static analysis (staticcheck)
  - Docker image build and smoke testing
  - Runs on: Push to `main`, PRs

- **[frontend.yml](frontend.yml)** - Web UI workflow
  - Build React/Vite application
  - Dependency checks
  - Bundle size analysis
  - Type checking (if configured)
  - Runs on: Push to `main`, PRs affecting web/

- **[integration-tests.yml](integration-tests.yml)** - E2E testing
  - Builds complete stack (backend + frontend)
  - Runs Playwright tests
  - Generates test reports
  - Runs on: Push to `main`, PRs, manual trigger

### Security & Quality

- **[codeql.yml](codeql.yml)** - Advanced security scanning
  - Analyzes Go and JavaScript/TypeScript code
  - Detects security vulnerabilities
  - Runs quality checks
  - Runs on: Push to `main`, PRs, weekly schedule, manual trigger

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
  - Runs on: Version tags (`v*.*.*`)

### Validation & Documentation

- **[validate-examples.yml](validate-examples.yml)** - Example skills validation
  - Validates skill.yaml structure
  - Checks required files (SKILL.md, VERSION, CHANGELOG.md)
  - Verifies SemVer format
  - Tests packaging
  - Runs on: Push to `main`, PRs affecting examples/

- **[openapi.yml](openapi.yml)** - API documentation
  - Validates OpenAPI specifications
  - Generates bundled spec
  - Creates ReDoc documentation
  - Detects breaking changes in PRs
  - Runs on: Push to `main`, PRs affecting OpenAPI files

- **[pr-checks.yml](pr-checks.yml)** - PR validation
  - Validates PR title format
  - Checks for merge conflicts
  - Validates commit messages
  - Detects large files
  - Scans for sensitive data patterns
  - Posts PR summary comment
  - Runs on: PR open, synchronize, reopen

## Automated Dependency Updates

- **[dependabot.yml](../dependabot.yml)** - Dependency management
  - Go modules (weekly)
  - npm packages for web UI (weekly)
  - npm packages for testing (weekly)
  - GitHub Actions (weekly)
  - Groups minor and patch updates

## Required Secrets

### For Releases

- `GITHUB_TOKEN` - Automatically provided by GitHub Actions
  - Used for: Creating releases, pushing Docker images, signing with cosign

### Optional Secrets

- `SLACK_WEBHOOK` - For build notifications (not configured)
- `CODECOV_TOKEN` - For coverage reporting (not configured)

## Status Badges

Add these to your README.md:

```markdown
![CI](https://github.com/YOUR_ORG/SkillForge/workflows/skill-registry/badge.svg)
![Frontend](https://github.com/YOUR_ORG/SkillForge/workflows/frontend/badge.svg)
![Integration Tests](https://github.com/YOUR_ORG/SkillForge/workflows/integration-tests/badge.svg)
![CodeQL](https://github.com/YOUR_ORG/SkillForge/workflows/codeql/badge.svg)
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
