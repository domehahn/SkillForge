# SkillForge

SkillForge is a self-hosted registry and lifecycle platform for AI Agent Skills.

Use `skcr` to scaffold or render local agent/project structures. Use SkillForge and `skforge` to package, publish, discover, install, pin, verify, deprecate, and govern skill artifacts.

This repository contains `skill-registry/`, the registry product implementation:

- registry server
- REST API
- Web UI
- `skforge` CLI
- content-addressable package storage
- SQLite metadata
- token authentication
- validation, lockfile, packaging, and governance workflows

## Responsibility Boundary

SkillForge / `skforge` owns artifact lifecycle:

- registry
- publish, search, info, install
- validate and package
- lock and verify
- deprecate, yank, unyank, delete
- token management and audit logging
- skill artifact governance

`skcr` owns local creation and rendering:

- scaffold skill skeletons
- scaffold project templates
- bake/render platform-specific project files
- manage `agentic.bake.yaml`
- manage `.agentic-template.lock`

SkillForge must not absorb `skcr bake`, project template rendering, or platform-specific project generation.

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

Local development:

```bash
cd skill-registry
make build
./bin/skill-registry
```

Publish:

```bash
skforge validate gitlab-policy-reviewer --strict
skforge package gitlab-policy-reviewer --output-dir dist --source-commit "$(git rev-parse HEAD)" --provenance
skforge publish dist/gitlab-policy-reviewer-1.5.0.tgz --registry http://localhost:8080
```

Consume in a project:

```bash
skforge init
skforge add default/gitlab-policy-reviewer@^1.5.0
skforge lock
skforge install --frozen-lockfile
skforge verify
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

SkillForge may grow toward more lifecycle features currently discussed for `skpm`, but the product boundary stays clear: SkillForge governs versioned artifacts; `skcr` renders local project structures.
