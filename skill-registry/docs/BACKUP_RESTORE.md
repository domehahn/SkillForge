# SkillForge Registry Backup & Restore Runbook

## Overview

SkillForge Registry manages critical skill artifact metadata (PostgreSQL) and immutable package binaries (S3/Object Storage). This document defines the backup architecture, operational procedures, Recovery Point Objective (RPO), Recovery Time Objective (RTO), and disaster recovery validation steps.

---

## 1. System Components & Scope

| Component | Technology | Primary State | Backup Mechanism |
| :--- | :--- | :--- | :--- |
| **Metadata & Auth** | PostgreSQL (or SQLite in dev/standalone) | Schema, users, ACLs, tokens, attestations, yanks, webhooks | Point-In-Time Recovery (PITR) / `pg_dump` snapshot |
| **Object Storage** | S3 / MinIO | Package blobs (`.tgz`), digests, attestations | S3 Cross-Region Replication (CRR) / Lifecycle backup |
| **Configuration** | `config.yaml` / Environment Variables | Secrets, TLS keys, DB URIs, S3 endpoints | Version-controlled GitOps repo & Secret Manager |

---

## 2. RPO and RTO Targets

- **Recovery Point Objective (RPO)**: **< 5 minutes**
  - PostgreSQL WAL (Write-Ahead Logging) archiving continuous sync.
  - S3 Versioning & Object Locking enabled.
- **Recovery Time Objective (RTO)**: **< 15 minutes**
  - Automated database restoration + S3 bucket policy re-mapping + health check verification (`/healthz`).

---

## 3. Backup Protection & Retention Policy

1. **Database Backups**:
   - **Continuous**: WAL archiving to S3 backup bucket.
   - **Daily Snapshots**: Automated `pg_dump` (compressed with gzip, encrypted with AES-256 / KMS).
   - **Retention**: Daily backups retained for 30 days; weekly backups retained for 90 days; monthly backups retained for 1 year.
2. **Object Storage Backups**:
   - S3 Bucket Versioning enabled on production blob buckets.
   - S3 Object Lock (WORM - Write Once Read Many) enabled for compliance & immutability protection.
   - S3 Cross-Region Replication (CRR) to a secondary cloud region.
3. **Encryption at Rest & in Transit**:
   - All backups encrypted using AWS KMS / HashiCorp Vault.
   - TLS 1.3 required for backup transport to remote storage.

---

## 4. Disaster Recovery & Restoration Procedure

### 4.1 Prerequisites
- Access to production backup storage (S3 bucket or snapshot store).
- Target PostgreSQL database instance (clean instance).
- Target S3 Object Storage bucket (configured with credentials matching `config.yaml`).

### 4.2 Step-by-Step Restoration

1. **Isolate Environment**:
   - Stop registry instances (`docker-compose down` or scale Kubernetes deployment to `0`).

2. **Restore PostgreSQL Database**:
   ```bash
   # Download latest encrypted database backup
   aws s3 cp s3://skillforge-backups-prod/db/latest.sql.gz.enc /tmp/latest.sql.gz.enc

   # Decrypt backup
   aws kms decrypt --ciphertext-blob fileb:///tmp/latest.sql.gz.enc --output text --query Plaintext | base64 --decode > /tmp/latest.sql.gz

   # Decompress and restore schema & data
   gunzip -c /tmp/latest.sql.gz | psql -h $DB_HOST -U $DB_USER -d skillforge
   ```

3. **Restore Object Storage Blobs**:
   ```bash
   # Sync S3 backup bucket to target storage bucket
   aws s3 sync s3://skillforge-backups-prod/blobs/ s3://skillforge-registry-blobs/ --delete
   ```

4. **Verify System Integrity & Restart Services**:
   - Scale up registry instances (`docker-compose up -d` or scale deployment to `N`).
   - Confirm migration startup lock runs cleanly (`dbmigrate`).
   - Run verification query to test `/healthz` and `/readyz`:
     ```bash
     curl -f http://localhost:8080/healthz
     curl -f http://localhost:8080/readyz
     ```

---

## 5. Automated Backup & Restore E2E Test

SkillForge includes an automated full-lifecycle backup and restore test suite (`tests/backup_restore_e2e_test.go`) that validates:
1. Artifact publishing & SHA-256 blob digest verification.
2. Namespace ACL permissions & API tokens survival.
3. Attestation attachment & metadata restoration.
4. Yanked artifact state retention.
5. Storage blob retrieval integrity post-restoration.

To execute the test locally or in CI:
```bash
go test -v ./tests -run TestBackupAndRestore_FullLifecycle
```

