# SkillForge

SkillForge is a self-hosted registry server for immutable AI Agent Skill artifacts.

Use `skcr` to scaffold and render local agent/project structures.
Use `skpm` to validate, package, publish, install, lock, update, and verify skills.
Use SkillForge as the self-hosted registry server for immutable skill artifacts.
Use `skforge` for SkillForge-specific administration, governance, and debugging.

This repository contains `skill-registry/`, the registry server implementation:

- registry server
- REST API
- Web UI
- `skforge` admin CLI
- content-addressable package storage
- SQLite metadata
- token authentication
- governance workflows (deprecate, yank, unyank, delete)

## Responsibility Boundary

SkillForge / `skforge` owns registry-server concerns:

- immutable artifact storage and content-addressed retrieval
- search, info, resolve, download, and publish API endpoints
- server-side publish validation and metadata consistency checks
- deprecate, yank, unyank, delete governance
- token management and audit logging
- namespace ACLs and webhooks

`skpm` owns consumer lifecycle:

- validate and lint skill packages locally
- format and package skills into ZIP/TGZ artifacts
- publish lifecycle orchestration (calls SkillForge API)
- `agent-skills.yaml` and `agent-skills.lock` management
- install, update, outdated, and verify workflows
- multi-registry client behavior

`skcr` owns local creation and rendering:

- scaffold skill skeletons and project templates
- bake/render platform-specific project files
- manage `agentic.bake.yaml` and `.agentic-template.lock`

SkillForge must not absorb `skcr bake`, project template rendering, or consumer-lifecycle commands (`install`, `lock`, `verify`) that belong to `skpm`.

## Versioning Model

SkillForge treats skills as versioned software artifacts, not loose Markdown files.

```text
1. Source versioning       -> Git
2. Skill versioning        -> SemVer per skill
3. Package versioning      -> immutable ZIP/TGZ/OCI-style registry artifact
4. Consumption versioning  -> projects pin resolved versions and checksums
```

`latest` is only for local experiments. Production projects must pin explicit versions and checksums through `agent-skills.lock`.

## Canonical Skill Layout

```text
<skill-name>/
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
entrypoint: SKILL.md
tags:
  - security
security:
  requires_network: false
  requires_secrets: false
  writes_files: false
  runs_commands: false
```

`VERSION` must contain the same SemVer as `skill.yaml.version`.

## Workflow

Start the registry server:

```bash
cd skill-registry
make build
./bin/skill-registry
```

Administer the registry with `skforge`:

```bash
skforge token create --name ci-publish --scopes write
skforge skills list
skforge skills deprecate default/gitlab-policy-reviewer 1.4.0 --reason "superseded by 1.5.0"
skforge skills yank default/gitlab-policy-reviewer 1.3.0 --reason "critical bug"
```

Publish a skill (use `skpm` for the full lifecycle):

```bash
# package and publish via skpm
skpm validate gitlab-policy-reviewer --strict
skpm package gitlab-policy-reviewer --output-dir dist --source-commit "$(git rev-parse HEAD)"
skpm publish dist/gitlab-policy-reviewer-1.5.0.tgz --registry http://localhost:8080
```

Consume skills in a project (use `skpm`):

```bash
skpm init
skpm add default/gitlab-policy-reviewer@^1.5.0
skpm lock
skpm install --frozen-lockfile
skpm verify
```

The lockfile records resolved versions, SHA-256 digests, registry URLs, and install targets.

## Supply-Chain Posture

Skills can run commands, read context, write files, or influence agent behavior. SkillForge therefore treats them like supply-chain artifacts:

- immutable published versions
- digest verification before install
- canonical metadata
- strict publish validation
- secret scanning heuristics
- path traversal and unsafe symlink rejection
- provenance/signing hooks
- audit logs for lifecycle actions
- deprecate/yank before destructive delete

## Roadmap Boundaries

The product boundary is fixed: SkillForge stores and serves versioned artifacts; `skpm` manages the consumer lifecycle; `skcr` renders local project structures. SkillForge will not absorb consumer-side packaging, lockfile management, or install workflows.
