# Production deployment checklist

This checklist defines the supported limited production profile: a private,
curated registry for an organization's own AI packages. It does not make
SkillForge an execution environment. Do not enable uploads from untrusted
publishers or automatically execute package contents. Runtime activation is
audited desired state; the registry does not start or isolate MCP servers or
containers.

## 1. Dependencies and network

- Use PostgreSQL, S3-compatible object storage, and Redis. `production.mode`
  rejects SQLite, local filesystem storage, in-memory rate limiting, disabled
  authentication, permissive CORS, missing TLS/proxy trust, and a missing
  stable webhook signing key.
- Use TLS directly in SkillForge or terminate it at a controlled ingress.
  When an ingress supplies forwarded client IP headers, trust only its exact
  source CIDR in `security.trusted_proxies`.
- Keep PostgreSQL, Redis, and the object store on private networks. Expose
  only the TLS registry endpoint. Restrict `/metrics` to the monitoring
  network because it is intentionally unauthenticated for Prometheus.
- Use separate least-privilege credentials for PostgreSQL, object storage,
  Redis, registry administrators, and CI publishers. Never use the repository
  demo Compose stack as a production topology.
- Set `SKILL_REGISTRY_WEBHOOK_SIGNING_KEY` to a stable value of at least 32
  bytes; production checks fail closed when it is missing or too short.
- Treat webhook destinations as an egress boundary. Restrict outbound traffic
  at the network layer to approved destinations and deny cloud metadata and
  control-plane ranges. `security.webhook_allowed_hosts` is an exact host or
  host:port allowlist; the server rejects other destinations, URL credentials,
  fragments, and redirects. Network egress controls remain necessary to
  protect against DNS rebinding and to limit non-webhook outbound paths.

## 2. Configure and check

Start from `config.example.production.yaml`. The loader expands
`${NAME}`, `${NAME:-default}`, and `${NAME?message}` placeholders. A required
unset variable fails configuration loading without printing its value.
Mount the TLS certificate and private key at the configured paths. Supply
secrets through a secret manager or orchestrator secret mounts. If webhooks
are used, include a stable `SKILL_REGISTRY_WEBHOOK_SIGNING_KEY` with at least
32 bytes. Keep that key identical across replicas and plan coordinated
rotation with webhook consumers.

On first startup, set `SKILL_REGISTRY_INITIAL_ADMIN_USER` and
`SKILL_REGISTRY_INITIAL_ADMIN_PASSWORD` through the secret manager to create
the initial administrator. Remove the bootstrap password from the deployment
after the account exists; later restarts do not need it.

Build the server and run the configuration check before deployment:

```bash
cd skill-registry
make build
./bin/skill-registry admin production-check --config ./config.example.production.yaml
```

The command checks application configuration only. It does not connect to or
prove the security, availability, backup configuration, or recoverability of
managed infrastructure. A configured `backup.managed: true` is an operator
acknowledgement, not evidence of an external backup.

## 3. Access and package admission

- Keep account creation and administrator access restricted. Use verified
  email or an organization-controlled identity gateway, and require MFA at
  that gateway where available.
- Give CI publisher tokens only `write` and the additional scope it needs;
  use separate read-only tokens for consumers. Rotate and revoke tokens on a
  documented schedule and after suspected exposure.
- Set `auth.public_read: false` so registry and skill read endpoints require
  a token with `read` scope. The production example enables this. Global
  health, capability, OpenAPI, and metrics endpoints remain available for
  probes/scrapers; restrict metrics at the network boundary.
- Restrict namespace maintainers to named, trusted publishers. Require human
  review before packages are consumed by production agents. Activation
  approval requires a separately authenticated actor from the publisher.
- Give each runtime controller a non-admin service account with maintainer
  membership only in namespaces it reconciles. Create a token with only
  `runtime:report`, set an expiry of at most 365 days, and rotate it on a
  documented schedule. Do not give a controller publish or activation-action
  permissions unless its duties require them.
- Production activation requires verified Ed25519 artifact signatures. Scan
  results also require Ed25519 signatures bound to the exact artifact digest
  and result. Configure separate publisher (`usage: artifact`) and scanner
  (`usage: scan`) public keys in `signatures.trusted_keys`; the registry
  enforces those roles. Keep private signing keys outside the registry and
  restrict scanner signing keys to the scanning service. The registry verifies
  evidence but does not execute scans or manage key custody and rotation.
- The upstream cache supports skills and typed artifacts. Configure explicit
  HTTPS origins and read-only bearer credentials for private upstreams. It
  verifies bytes and metadata before local caching; it does not import remote
  signature trust. Enforce outbound network policy at the host or network layer.

## 4. Backup, upgrade, and rollback

For PostgreSQL and S3, configure provider-managed database recovery (including
PITR where available), object versioning, and a separate recovery location.
Keep database metadata and object blobs recoverable together. Follow
[`BACKUP_RESTORE.md`](BACKUP_RESTORE.md) and complete an isolated restore
exercise before accepting production traffic. Record the measured recovery
point and duration; do not claim an RPO/RTO until the operator has verified it.

Before every upgrade:

1. Review release notes and the schema changes for the exact image digest.
2. Confirm a recent recoverable database backup and object-store recovery
   point.
3. Deploy one new instance first and wait for `/readyz` before shifting
   traffic. Do not run mixed versions unless that release explicitly documents
   compatibility.
4. Check artifact search, resolve, upload, and digest-verified download using
   a non-production canary namespace.
5. Keep the prior image digest available. If startup, migration, or canary
   checks fail, remove it from traffic and restore the database and object
   store together according to the provider runbook. Schema rollback is not
   automatic.

## 5. Monitoring and incident response

Probe `/healthz` for process liveness and `/readyz` for database, storage, and
rate-limit dependency readiness. Scrape `/metrics` only from the monitoring
network. Alert on readiness failures, sustained 5xx responses, latency against
the service's agreed SLO, database/storage errors, failed provider backups,
and backup-age or restore-test expiry.

Maintain an operator-owned dashboard and alert route with an on-call owner.
The repository exposes HTTP request/error/duration metrics but does not ship
deployment-specific dashboards, alert thresholds, paging routes, or an
incident contact. Record who can disable publishing, revoke a token, block a
namespace, and restore service during an incident.

## Go-live evidence

Do not declare a deployment ready until its owner has recorded:

- the output of `admin production-check` for the deployed config (without
  storing secret values);
- a successful TLS probe and least-privilege access review;
- a successful canary publish, resolve, and digest-verified download;
- an isolated backup restore and the measured recovery point/time;
- monitoring alerts delivered to the on-call route; and
- the release image digest, rollback owner, and incident contact.
