# GitHub Actions Setup Summary

## ✅ What's Been Added

I've added a comprehensive GitHub Actions CI/CD setup for SkillForge with 7 new workflows, plus Dependabot configuration and supporting files.

### New Workflows

#### 1. **frontend.yml** - Web UI Build & Test
- Builds the React/Vite web application
- Type checking support
- Bundle size analysis
- Dependency auditing
- **Triggers**: Push to main, PRs affecting `skill-registry/web/`

#### 2. **integration-tests.yml** - End-to-End Testing
- Builds complete stack (Go backend + React frontend)
- Runs Playwright E2E tests
- Generates test reports with screenshots/videos on failure
- **Triggers**: Push to main, PRs, manual dispatch

#### 3. **codeql.yml** - Advanced Security Scanning
- Analyzes Go and JavaScript/TypeScript code
- Detects security vulnerabilities and code quality issues
- Runs security-and-quality query suite
- **Triggers**: Push to main, PRs, weekly schedule (Monday 10:00 UTC), manual

#### 4. **pr-checks.yml** - Pull Request Validation
- Validates PR title format (conventional commits)
- Checks for merge conflicts
- Validates commit messages
- Detects large files (>1MB)
- Scans for potential sensitive data
- Posts helpful PR summary comment
- **Triggers**: PR open, synchronize, reopen

#### 5. **validate-examples.yml** - Example Skill Validation
- Validates skill.yaml structure
- Checks required files (SKILL.md, VERSION, CHANGELOG.md, README.md)
- Verifies SemVer format in VERSION files
- Validates YAML frontmatter
- Tests skill packaging
- **Triggers**: Push to main, PRs affecting `skill-registry/examples/`, manual

#### 6. **openapi.yml** - API Documentation
- Lints OpenAPI specifications
- Bundles spec files
- Generates ReDoc documentation
- Detects breaking changes in PRs
- Optionally deploys to GitHub Pages
- **Triggers**: Push to main, PRs affecting OpenAPI files, manual

#### 7. **dependabot.yml** - Automated Dependency Updates
- Weekly updates for Go modules
- Weekly updates for npm packages (web UI and tests)
- Weekly updates for GitHub Actions
- Groups minor and patch updates to reduce noise
- Automatic labeling and conventional commit messages

### Supporting Files

#### 8. **playwright.config.js** - E2E Test Configuration
- Configures Playwright test runner
- Sets up reporters (HTML, JSON, GitHub)
- Configures retries and parallel execution
- Screenshot and video capture on failures

#### 9. **.github/workflows/README.md** - Documentation
- Comprehensive documentation of all workflows
- Trigger reference table
- Local testing instructions
- Troubleshooting guide
- Status badge examples

#### 10. **Updated README.md**
- Added workflow status badges
- Links to GitHub Actions

## 📊 Workflow Coverage

Your CI/CD now covers:

| Area | Coverage |
|------|----------|
| ✅ Backend (Go) | Tests, linting, race detection, vulnerability scanning, static analysis |
| ✅ Frontend (React) | Build, type checking, bundle analysis |
| ✅ Integration | E2E tests with Playwright |
| ✅ Security | CodeQL, govulncheck, Trivy, weekly scans |
| ✅ Dependencies | Automated weekly updates |
| ✅ Documentation | OpenAPI validation, ReDoc generation |
| ✅ Examples | Skill artifact validation |
| ✅ Releases | Cross-platform builds, Docker images, signing, SBOM |

## 🚀 Next Steps

### 1. Update Repository Settings

Replace `YOUR_ORG` in the status badges with your actual GitHub organization/username:

```bash
# In README.md, replace:
YOUR_ORG → your-github-username
```

### 2. Enable GitHub Actions Permissions

Go to: **Settings → Actions → General**

Ensure these permissions are enabled:
- ✅ Read and write permissions (for releases)
- ✅ Allow GitHub Actions to create and approve pull requests (for Dependabot)

### 3. Enable Security Features

Go to: **Settings → Code security and analysis**

Enable:
- ✅ Dependabot alerts
- ✅ Dependabot security updates
- ✅ CodeQL analysis
- ✅ Secret scanning

### 4. Optional: Enable GitHub Pages (for API docs)

If you want to publish API documentation:

1. Go to **Settings → Pages**
2. Source: Deploy from a branch
3. Branch: `gh-pages` / root
4. In `.github/workflows/openapi.yml`, change `if: false` to `if: true` in the Deploy step

### 5. Test the Workflows

#### Test locally with act (optional):
```bash
# Install act
brew install act

# Test backend workflow
act -W .github/workflows/skill-registry.yml

# Test frontend workflow
act -W .github/workflows/frontend.yml
```

#### Test on GitHub:
```bash
# Create a feature branch
git checkout -b test/github-actions

# Push to trigger workflows
git push origin test/github-actions

# Create a PR to test PR-specific workflows
gh pr create --title "test: GitHub Actions setup"
```

### 6. Create Your First Release

When ready to create a release:

```bash
# Tag with semantic version
git tag v0.1.0

# Push the tag
git push origin v0.1.0
```

This will trigger the release workflow which will:
- Build cross-platform binaries (Linux, macOS, Windows × amd64, arm64)
- Create Docker multi-platform images
- Sign images with cosign
- Generate SBOM
- Create GitHub Release with all artifacts

## 📝 Workflow Status

You can monitor workflow runs at:
```
https://github.com/YOUR_ORG/SkillForge/actions
```

## 🔧 Customization

### Adjust Schedules

Weekly scans run on Monday. To change:

**CodeQL** (`.github/workflows/codeql.yml`):
```yaml
schedule:
  - cron: '0 10 * * 1'  # Monday 10:00 UTC
```

**Base Image Refresh** (`.github/workflows/base-image-refresh.yml`):
```yaml
schedule:
  - cron: '0 6 * * 1'  # Monday 06:00 UTC
```

**Dependabot** (`.github/dependabot.yml`):
```yaml
schedule:
  interval: "weekly"
  day: "monday"
```

### Add More Browsers to E2E Tests

Edit `skill-registry/playwright.config.js`:
```javascript
projects: [
  { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
  { name: 'firefox', use: { ...devices['Desktop Firefox'] } },  // Uncomment
  { name: 'webkit', use: { ...devices['Desktop Safari'] } },     // Uncomment
]
```

### Add Slack Notifications

Add to any workflow:
```yaml
- name: Notify Slack
  if: failure()
  uses: slackapi/slack-github-action@v1
  with:
    webhook: ${{ secrets.SLACK_WEBHOOK }}
    payload: |
      {
        "text": "Build failed: ${{ github.workflow }}"
      }
```

## 🐛 Troubleshooting

### Frontend build fails
Ensure `package-lock.json` exists:
```bash
cd skill-registry/web
npm install
git add package-lock.json
git commit -m "chore: add package-lock.json"
```

### Integration tests timeout
Increase timeout in `.github/workflows/integration-tests.yml`:
```yaml
timeout-minutes: 30  # Increase from 15
```

### Docker build fails
Verify Dockerfile path in workflows matches your structure:
```yaml
file: SkillForge/skill-registry/Dockerfile
```

## 📚 Resources

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Dependabot Configuration](https://docs.github.com/en/code-security/dependabot)
- [CodeQL Setup](https://codeql.github.com/)
- [Playwright Documentation](https://playwright.dev/)
- [act - Run Actions Locally](https://github.com/nektos/act)

## ✨ What's Already There

The existing workflows are excellent and remain unchanged:
- ✅ **skill-registry.yml** - Backend CI with comprehensive testing
- ✅ **release.yml** - Production release pipeline
- ✅ **base-image-refresh.yml** - Weekly security scanning

## 🎉 Summary

You now have a production-ready CI/CD pipeline that:
- ✅ Tests both backend and frontend
- ✅ Runs integration tests
- ✅ Performs security scanning
- ✅ Validates PRs and examples
- ✅ Automates dependency updates
- ✅ Generates API documentation
- ✅ Creates cross-platform releases
- ✅ Signs and validates Docker images
- ✅ Generates SBOMs for compliance

All workflows follow best practices and are ready to use!
