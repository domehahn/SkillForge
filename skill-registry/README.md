# SkillForge Registry

SkillForge Registry is the server, REST API, Web UI, and `skforge` CLI for a self-hosted AI Agent Skill registry.

SkillForge is deliberately registry-shaped: close to Verdaccio, npm Registry, and Docker Hub, but focused on versioned AI agent artifacts. It stores immutable skill packages, validates canonical metadata, tracks digests, exposes discovery APIs, supports lifecycle governance, and installs through lockfiles.

Use `skcr` to scaffold skills or render local project structures. Use SkillForge/`skforge` to package, publish, discover, install, pin, verify, deprecate, yank, and govern skill artifacts.

## Versioning Model

```text
1. Source versioning       -> Git
2. Skill versioning        -> SemVer per skill
3. Package versioning      -> immutable ZIP/TGZ/OCI-style registry artifact
4. Consumption versioning  -> projects pin resolved versions and checksums
```

`latest` is only for local experiments. Production projects must pin explicit versions and checksums through `agent-skills.lock`.

## Canonical Skill Layout

```text
gitlab-policy-reviewer/
├── SKILL.md
├── VERSION
├── skill.yaml
├── CHANGELOG.md
├── README.md
├── examples/
└── tests/
```

Publish-grade validation requires:

- `SKILL.md`
- `VERSION`
- `skill.yaml`
- `CHANGELOG.md`

Recommended:

- `README.md`
- `examples/`
- `tests/`
- `LICENSE`

`skill.yaml` is authoritative:

```yaml
name: gitlab-policy-reviewer
namespace: default
version: 1.5.0
description: Reviews GitLab security policies.
owners:
  - platform-security
license: MIT
compatible_with:
  - codex
  - gitlab-duo
  - github-copilot
  - claude-code
entrypoint: SKILL.md
tags:
  - security
  - gitlab
  - compliance
security:
  requires_network: false
  requires_secrets: false
  writes_files: false
  runs_commands: false
```

`VERSION`:

```text
1.5.0
```

`CHANGELOG.md`:

```markdown
# Changelog

## 1.5.0

### Added
- Added initial skill implementation.
```

## Quickstart

```bash
# 1. Create a skill skeleton, optionally via skcr
skcr scaffold skill gitlab-policy-reviewer

# 2. Validate
skforge validate gitlab-policy-reviewer --strict

# 3. Package deterministically
skforge package gitlab-policy-reviewer --output-dir dist --source-commit "$(git rev-parse HEAD)" --provenance

# 4. Publish
skforge publish dist/gitlab-policy-reviewer-1.5.0.tgz --registry http://localhost:8080

# 5. Consume in a project
skforge init
skforge add default/gitlab-policy-reviewer@^1.5.0
skforge lock
skforge install --frozen-lockfile
skforge verify
```

If `skcr` is unavailable, create the canonical layout manually. Do not publish a package containing only `SKILL.md`.

## CLI

Build:

```bash
make build
```

Run server:

```bash
./bin/skill-registry
```

Package:

```bash
skforge package <skill-dir> \
  --output-dir dist \
  --format tgz \
  --source-commit "$(git rev-parse HEAD)" \
  --provenance
```

Validation profiles:

```bash
skforge validate <path-or-archive>
skforge validate <path-or-archive> --strict
skforge validate <path-or-archive> --publish
```

Publish:

```bash
skforge publish dist/gitlab-policy-reviewer-1.5.0.tgz --registry http://localhost:8080
```

Expected output:

```text
Published default/gitlab-policy-reviewer@1.5.0
Digest: sha256:<digest>
Package: gitlab-policy-reviewer-1.5.0.tgz
```

Metadata flags such as `--namespace`, `--name`, and `--version` are consistency checks by default. If a flag does not match `skill.yaml`/`VERSION`, publish fails.

## Lockfile Workflow

`agent-skills.yaml` records desired skills and constraints:

```yaml
registries:
  default:
    url: http://localhost:8080

skills:
  - name: gitlab-policy-reviewer
    namespace: default
    version: ^1.5.0
    registry: default
    target: .agents/skills/gitlab-policy-reviewer
```

`agent-skills.lock` records resolved versions, digests, registries, and install paths:

```yaml
lockfile_version: 1
generated_at: "2026-06-08T00:00:00Z"
skills:
  - name: gitlab-policy-reviewer
    namespace: default
    version: 1.5.2
    constraint: ^1.5.0
    registry: default
    registry_url: http://localhost:8080
    artifact: gitlab-policy-reviewer-1.5.2.tgz
    sha256: "..."
    compatible_with:
      - codex
      - gitlab-duo
    installed_to:
      - .agents/skills/gitlab-policy-reviewer
```

Commands:

```bash
skforge init
skforge add default/gitlab-policy-reviewer@^1.5.0
skforge remove default/gitlab-policy-reviewer
skforge lock
skforge install --frozen-lockfile
skforge install --check
skforge verify
skforge outdated
skforge update
```

## Governance

Prefer lifecycle actions before destructive deletion:

```bash
skforge deprecate default/gitlab-policy-reviewer@1.5.0 --reason "Use 1.5.2"
skforge yank default/gitlab-policy-reviewer@1.5.1 --reason "Bad metadata"
skforge unyank default/gitlab-policy-reviewer@1.5.1
```

Semantics:

- `deprecate`: version remains installable but clients and UI show a warning.
- `yank`: version is hidden from new resolution; exact locked installs may still work by policy.
- `delete`: admin-only emergency operation. It is not the primary lifecycle path.

## API

Useful endpoints:

```text
GET /api/v1/capabilities
GET /api/v1/skills
GET /api/v1/skills/{namespace}/{name}
GET /api/v1/skills/{namespace}/{name}/resolve?constraint=^1.5.0
PUT /api/v1/skills/{namespace}/{name}/versions/{version}
POST /api/v1/skills/{namespace}/{name}/versions/{version}/deprecate
POST /api/v1/skills/{namespace}/{name}/versions/{version}/yank
POST /api/v1/skills/{namespace}/{name}/versions/{version}/unyank
```

The generic Artifact API also supports `skill`, `agent`, `flow`, `prompt`, `tool`, and `bundle` artifacts with dependency graphs, lockfiles, promotions, namespace ACLs, webhooks, attestations, and OCI descriptors. See [docs/ARTIFACTS.md](docs/ARTIFACTS.md).

## Security Posture

SkillForge treats skills as supply-chain artifacts:

- no secrets in packages
- no unsafe paths or symlinks
- no `.git`, dependency folders, caches, or build outputs in generated packages
- digest verification before extraction
- immutable versions by default
- signing/provenance-ready metadata
- audit logging for publish, yank, delete, and token actions
- clear token scopes

## Migration

Old minimal skill:

```text
SKILL.md
```

New production skill:

```text
SKILL.md
VERSION
skill.yaml
CHANGELOG.md
README.md
```

Default validation may warn on old packages. Publish validation fails until the canonical metadata files are added.

## Docker

```bash
docker compose up -d --build
curl http://localhost:8080/healthz
```

The Web UI is served from the registry container when `web/dist` is present.
