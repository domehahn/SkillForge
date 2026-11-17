# Local Agent Registry Platform

Status: implementation
Owner: platform-engineering
Source: user request dated 2026-11-16 and the HarnessMesh architecture review

## Scope

SkillForge becomes the authoritative local catalog and desired-state registry
for skills, MCP servers, agent instructions, hooks, agents, plugins, policies,
containerized model runtimes, and model profiles. Runtime execution remains a
separate, least-privileged control-plane concern.

## Requirements and acceptance criteria

| ID | Requirement | Acceptance criterion |
| --- | --- | --- |
| LR-01 | Version all supported component types as immutable artifacts. | The API accepts and validates `instructions`, `hook`, `plugin`, `policy`, `container`, `model-runtime`, and `model-profile` in addition to existing kinds. |
| LR-02 | Represent AGENTS.md-style instructions portably. | An instructions manifest declares format, scope, target platforms, target path, precedence, and merge strategy. Unsafe target paths are rejected. |
| LR-03 | Represent agent lifecycle hooks safely. | A hook declares a supported event, matcher, command/image, timeout, capabilities, failure policy, and approval requirement; executable hooks default to approval-required. |
| LR-04 | Register MCP servers as untrusted extensions. | MCP manifests declare transport, tool/resource metadata, policy controls, secret references, health configuration, and optional container image. Literal values for secret-like environment keys are rejected. |
| LR-05 | Register local model containers without storing model weights in SkillForge packages. | Model runtime manifests use digest-pinned OCI images and resource/health/network constraints; model profiles reference a runtime artifact and model metadata. |
| LR-06 | Separate publishing from execution. | Publishing never executes an artifact. Tenant-scoped activation records use explicit discovered/installed/approved/active/disabled/failed/rollback states. |
| LR-07 | Make activation auditable and reversible. | State changes record actor, desired/observed state, generation, reason, timestamps, and prior version; rollback selects the prior version without deleting artifacts. |
| LR-08 | Enforce admission before activation. | Executable artifacts require valid validation state, a non-failed scan, and approval; deployments may additionally require a verified signature. |
| LR-09 | Avoid credential material in artifacts and logs. | Manifests contain secret references only. APIs never return resolved secret values. |
| LR-10 | Provide a local runtime deployment path. | Compose offers opt-in OCI registry, Ollama, and model-gateway profiles bound to loopback with persistent model storage and health checks. |
| LR-11 | Expose the complete contract through API, CLI, UI, and documentation. | Metadata capabilities, OpenAPI, publish guidance, artifact cards, and CLI commands include the new artifact and activation features. |
| LR-12 | Preserve supply-chain evidence. | Content digests, dependency locks, OCI descriptors, signed attestations, scan status, and activation audit events remain tied to an immutable version. |

## Security invariants

- No artifact is executed as a side effect of publish, install, approval, or API
  reads.
- Container references used for activation are digest-pinned.
- Secret-like environment variables may only reference a named secret.
- Activation mutations require namespace maintainer or owner authorization.
- Approval and activation are distinct transitions.
- Webhook signatures use per-deployment configured key material; there is no
  built-in shared signing secret.
- Runtime services bind to loopback by default and do not mount the Docker
  socket into the registry.

## Ownership boundaries

- SkillForge owns catalog metadata, desired activation state, governance,
  evidence, and audit history.
- `skpm` owns artifact download, lock, verification, and local installation.
- `skcr` owns platform-specific rendering of AGENTS.md and related files.
- A runtime controller owns reconciliation with Docker/Podman/Kubernetes.
- OCI registry/model caches own image layers and model weights.
- HarnessMesh may consume SkillForge artifacts as an optional orchestration
  and policy runtime; its session/job system is not embedded in SkillForge.

## Rollback

Database additions are additive. New API routes can be removed without
affecting existing skill/artifact routes. Runtime Compose profiles are opt-in.
Activation can be disabled globally, and each activation can return to the
disabled state or select its recorded previous version.
