# SkillForge

[![Registry CI](https://github.com/domehahn/SkillForge/actions/workflows/skill-registry.yml/badge.svg)](https://github.com/domehahn/SkillForge/actions/workflows/skill-registry.yml)
[![Frontend CI](https://github.com/domehahn/SkillForge/actions/workflows/frontend.yml/badge.svg)](https://github.com/domehahn/SkillForge/actions/workflows/frontend.yml)
[![Integration Tests](https://github.com/domehahn/SkillForge/actions/workflows/integration-tests.yml/badge.svg)](https://github.com/domehahn/SkillForge/actions/workflows/integration-tests.yml)
[![CodeQL](https://github.com/domehahn/SkillForge/actions/workflows/codeql.yml/badge.svg)](https://github.com/domehahn/SkillForge/actions/workflows/codeql.yml)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/domehahn/SkillForge/blob/main/LICENSE)

SkillForge is a self-hosted package registry for AI development assets. Publish, version, discover, and distribute reusable agent packages from infrastructure you control. Packages are immutable, validated, digest-addressed, and governed by namespace permissions and an auditable release history.

Use `skcr` to scaffold and render local agent/project structures.
Use `skpm` to validate, package, publish, install, lock, update, and verify skills.
Use `skil` to scan skills for security issues and produce attestable evidence.
Use SkillForge as the self-hosted package registry for immutable AI artifacts.
Use `skforge` for SkillForge-specific administration, governance, and debugging.

The registry data model supports skills, agents, workflows, prompts, tools, bundles, MCP servers, instruction files, hooks, plugins, policies, containers, and local model runtime/profile definitions. `skpm` currently provides the consumer package lifecycle for skills. Runtime activation is represented as audited desired state; a separate controller is responsible for starting and supervising workloads.

This repository contains `skill-registry/`, the registry server, CLI, API, and web application:

- registry server
- REST API
- Web UI
- `skforge` admin CLI
- content-addressable filesystem or S3-compatible package storage
- SQLite or PostgreSQL metadata
- token authentication
- governance workflows (deprecate, yank, unyank, delete)

## Quick start

```bash
cd skill-registry
make build
./bin/skill-registry
```

The server binds to `127.0.0.1:8080` by default. Check readiness with
`curl http://localhost:8080/readyz`. For a containerized setup, see
[`skill-registry/README.md`](skill-registry/README.md#docker).

## Project map

| Path | Purpose |
| --- | --- |
| `skill-registry/internal/api` | HTTP routes, handlers, OpenAPI contract |
| `skill-registry/internal/registry` | Artifact lifecycle and policy decisions |
| `skill-registry/internal/metadata` | Persistence models and repository |
| `skill-registry/internal/storage` | Filesystem and S3-compatible package storage |
| `skill-registry/web` | React/Vite registry interface |
| `skill-registry/cmd` | Registry server and `skforge` CLI |
| `.github/workflows` | CI, security, integration, and release workflows |

## Project documentation

- [Registry setup and CLI](skill-registry/README.md)
- [Artifact formats and lifecycle](skill-registry/docs/ARTIFACTS.md)
- [Backup and restore](skill-registry/docs/BACKUP_RESTORE.md)
- [Production deployment](skill-registry/docs/PRODUCTION_DEPLOYMENT.md)
- [Contributing](CONTRIBUTING.md)
- [Security reporting](SECURITY.md)

## Responsibility Boundary

SkillForge / `skforge` owns registry-server concerns:

- immutable artifact storage and content-addressed retrieval
- search, info, resolve, download, and publish API endpoints
- server-side publish validation and metadata consistency checks
- deprecate, yank, unyank, delete governance
- token management and audit logging
- namespace ACLs and webhooks
- attestation storage (`/artifacts/{kind}/.../attestations`) — a generic,
  opaque-predicate record store that `skpm attest`/`attestations` use to
  attach `skil`-produced evidence to a published skill version; SkillForge
  does not interpret the predicate or run any analysis itself

`skpm` owns consumer lifecycle:

- validate and lint skill packages locally
- format and package skills into ZIP/TGZ artifacts
- publish lifecycle orchestration (calls SkillForge API)
- `agent-skills.yaml` and `agent-skills.lock` management
- install, update, outdated, and verify workflows
- multi-registry client behavior
- attaching third-party attestations (e.g. `skil` evidence) to a published
  version via SkillForge's attestation API

`skcr` owns local creation and rendering:

- scaffold skill skeletons and project templates
- bake/render platform-specific project files
- manage `agentic.bake.yaml` and `.agentic-template.lock`

`skil` owns security and assurance:

- static/semantic security analysis of a skill's contents
- declared-vs-observed capability verification
- policy decisions, evidence, and signed attestation
- does not touch package versioning, publishing, or the registry — its
  output can optionally be attached to a SkillForge-hosted version via
  `skpm attest`, but `skil` itself has no dependency on this repo or `skpm`

SkillForge must not absorb `skcr bake`, project template rendering, security
analysis that belongs to `skil`, or consumer-lifecycle commands (`install`,
`lock`, `verify`) that belong to `skpm`. SkillForge previously shipped a
duplicate `skpm` reimplementation under `skill-registry/cmd/skpm`; it has
been removed — `skpm` is the only package-manager CLI in this toolchain.

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

### Registry priorities

1. **Upstream registries and local caching.** Skill API requests can fetch configured registries on cache miss, verify metadata and package digests, and retain immutable copies. Generic artifact proxying, failure caching, and deployment egress allowlists remain open.
2. **A consistent package experience across artifact types.** Document and stabilize publish, search, resolve, download, and version behavior for the supported AI artifact types; provide client support where an artifact format needs it. Generic artifact storage does not by itself mean every client can consume every type.
3. **Verifiable package provenance.** Ed25519 artifact and scan attestations are verified against use-scoped deployment trust keys before status changes or activation. Remote upstream signatures are not imported as verified; broader provenance formats and key lifecycle automation remain open.
4. **Operational proof.** Exercise backup and restore against a real deployment, and define deployment-specific monitoring, alerting, rollback, and recovery targets before relying on the registry for critical distribution.

Starting or supervising agent containers, executing MCP servers, and rendering files into every agent platform remain outside the registry server's scope. Those responsibilities belong to dedicated clients and runtime controllers.
