# SkillForge Artifacts

SkillForge stores reusable agent ecosystem components as immutable, versioned artifacts.

Supported kinds:

- `skill`
- `agent`
- `flow`
- `prompt`
- `tool`
- `bundle`

## Manifest

Every generic artifact contains an `artifact.yaml`:

```yaml
apiVersion: skillforge.dev/v1
kind: agent
metadata:
  namespace: platform
  name: reviewer
  version: 1.0.0
  visibility: internal
spec:
  entrypoint: AGENT.md
  runtime: openai
  dependencies:
    - kind: skill
      namespace: platform
      name: documentation-review
      version: ^1.0.0
```

Dependencies support exact versions, dist-tags, `^` major-compatible ranges, `~` minor-compatible ranges, and optional dependencies.

Publishing resolves dependencies recursively and stores a digest-pinned lockfile. Cyclic dependencies fail validation.

## CLI

```bash
skforge artifact init agent ./reviewer-agent
skforge artifact publish skill demo/documentation-review@1.0.0 ./examples/hello-skill
skforge artifact publish agent demo/reviewer-agent@1.0.0 ./examples/reviewer-agent
skforge artifact publish flow demo/review-flow@1.0.0 ./examples/review-flow
skforge artifact graph flow demo/review-flow@0.1.0
skforge artifact lock flow demo/review-flow@0.1.0
skforge artifact promote flow demo/review-flow@1.0.0 production
skforge artifact attest flow demo/review-flow@1.0.0 scan sha256:...
skforge artifact install flow demo/review-flow@production ./installed-artifacts
skforge namespace member demo alice owner
skforge webhook add demo https://example.com/hook artifact.published,artifact.promoted
```

## Governance

Namespace memberships use `reader`, `maintainer`, and `owner` roles. Webhook registrations, promotion history, signature status, scan status, and OCI-compatible descriptors are stored alongside artifact metadata.

Webhook delivery currently emits JSON payloads for `artifact.published` and `artifact.promoted` with an `X-SkillForge-Event` header.
