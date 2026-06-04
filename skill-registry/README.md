# Skill Registry

A production-ready, lightweight self-hosted registry for versioned AI Agent Skills, inspired by Verdaccio but designed specifically for AI agent ecosystems.

## Overview

Skill Registry provides a centralized location for teams to publish, discover, and install reusable AI Agent Skills. It's designed to work with various skill formats including Codex skills, Claude skills, and generic agent skills.

## Features

- ✅ **Publishing**: Upload and version skill packages (zip/tar.gz)
- ✅ **Discovery**: Search and list available skills
- ✅ **Version Management**: Semantic versioning with "latest" alias support
- ✅ **Validation**: Comprehensive security and structure validation
- ✅ **Storage**: Content-addressable blob storage with SQLite metadata
- ✅ **Authentication**: Token-based auth with configurable scopes
- ✅ **Audit Logging**: Track all publish/delete operations
- ✅ **CLI Tool**: `skforge` for easy interaction
- ✅ **Web UI**: Modern React interface for browsing skills (like Verdaccio)
- ✅ **API-First**: REST API with OpenAPI specification
- ✅ **Docker Support**: Production-ready containers
- 🚧 **Upstream Proxy**: Cache remote registries (interface ready)

## Quick Start

### 🚀 Schnellstart (Entwicklung)

Der einfachste Weg für lokale Entwicklung:

```bash
# 1. Repository klonen
git clone https://github.com/skillforge/skill-registry
cd skill-registry

# 2. Go-Binaries bauen (Server + CLI)
make build

# 3. Optional: Web-UI bauen (für vollständige Erfahrung)
make build-web

# 4. Server starten (Standard: http://localhost:8080)
./bin/skill-registry
```

**Das war's!** Der Server läuft jetzt mit:
- ✅ REST API auf `http://localhost:8080/api/v1`
- ✅ Web-UI auf `http://localhost:8080` (falls gebaut)
- ✅ Health-Check auf `http://localhost:8080/healthz`

### 📦 Produktion mit Docker Compose

```bash
# Repository klonen
git clone https://github.com/skillforge/skill-registry
cd skill-registry

# Mit docker-compose starten
docker-compose up -d

# Admin-User erstellen (einmalig)
docker compose exec registry /app/skill-registry admin create-user \
  --username admin \
  --password admin123 \
  --role admin

# Registry läuft auf http://localhost:8080
# Login via: skforge login (admin / admin123)
```

### 🐳 Docker (Standalone)

```bash
# Image bauen
make docker-build

# Container starten
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  --name skill-registry \
  skill-registry:latest
```

## 📖 Erste Schritte Tutorial

### Schritt 1: Server starten

```bash
# Server bauen und starten
make build
./bin/skill-registry

# Überprüfen, ob der Server läuft
curl http://localhost:8080/healthz
# Erwartet: {"status":"ok"}
```

### Schritt 2: CLI einrichten & Login

```bash
# CLI ist bereits gebaut (./bin/skforge)
# Optional: In PATH verschieben
sudo mv ./bin/skforge /usr/local/bin/

# Erstelle einen Admin-User (server-seitig, einmalig)
./bin/skill-registry admin create-user \
  --username admin \
  --password admin123 \
  --role admin

# Login mit Username/Password
skforge login
# Registry URL: http://localhost:8080
# Username: admin
# Password: admin123
# ✅ Login successful!
```

**Hinweis:** Bei aktivierter Auth (`SKILL_REGISTRY_AUTH_ENABLED=true`) ist Login erforderlich für `publish` und `delete`.
Download und Search funktionieren immer ohne Login.
# Token: (leer lassen wenn Auth deaktiviert)
```

### Schritt 3: Ersten Skill erstellen

Ein Skill ist ein Verzeichnis mit mindestens einer `SKILL.md` Datei:

```bash
# Skill-Verzeichnis erstellen
mkdir my-first-skill
cd my-first-skill

# SKILL.md erstellen
cat > SKILL.md << 'EOF'
# My First Skill

This is my first skill for the registry.

## Usage

This skill demonstrates how to create a basic skill package.

## Example

```yaml
name: my-first-skill
namespace: default
version: 1.0.0
description: A simple example skill
tags:
  - example
  - tutorial
```
EOF

# Optional: Weitere Dateien hinzufügen
mkdir references
echo "# Reference docs" > references/README.md
```

### Schritt 4: Skill hochladen

```bash
# Vom Skill-Verzeichnis aus publishen
cd my-first-skill
skforge publish . --registry http://localhost:8080

# Oder als Archiv
cd ..
tar -czf my-first-skill.tar.gz my-first-skill/
skforge publish my-first-skill.tar.gz --registry http://localhost:8080

# Mit spezifischer Version und Namespace
skforge publish my-first-skill/ \
  --registry http://localhost:8080 \
  --namespace myteam \
  --name my-skill \
  --version 1.0.0
```

**Erwartete Ausgabe:**
```
✓ Skill erfolgreich veröffentlicht
  Namespace: default
  Name: my-first-skill
  Version: 1.0.0
  Digest: sha256:abc123...
```

### Schritt 5: Skills durchsuchen

**Via Web-UI:**
1. Browser öffnen: `http://localhost:8080`
2. Skills durchsuchen oder suchen
3. Auf einen Skill klicken für Details
4. Version herunterladen mit Download-Button

**Via CLI:**
```bash
# Alle Skills auflisten
skforge search

# Nach Stichwort suchen
skforge search "tutorial"

# Skill-Details anzeigen
skforge info default/my-first-skill
```

**Via API:**
```bash
# Alle Skills auflisten
curl http://localhost:8080/api/v1/skills

# Skill-Details abrufen
curl http://localhost:8080/api/v1/skills/default/my-first-skill

# Skill herunterladen
curl -O http://localhost:8080/api/v1/skills/default/my-first-skill/versions/1.0.0/download
```

### Schritt 6: Skill installieren

```bash
# In Standard-Verzeichnis installieren (~/.agents/skills/)
skforge install default/my-first-skill@1.0.0

# In custom Verzeichnis installieren
skforge install default/my-first-skill@1.0.0 --target ./my-skills

# Neueste Version installieren
skforge install default/my-first-skill@latest
```

## 🛠️ Häufig verwendete Befehle

### Makefile Targets

```bash
# Go-Binaries bauen (Server + CLI)
make build

# Nur Server bauen
make build-server

# Web-UI Dependencies installieren
make install-web

# Web-UI für Produktion bauen
make build-web

# Alles bauen (Go + Web)
make build-all

# Tests ausführen
make test

# Linting
make lint

# Server starten (baut vorher)
make run

# Web-UI Dev-Server starten
make dev-web

# Aufräumen
make clean

# Docker Image bauen
make docker-build

# Mit Docker Compose starten
make compose-up

# Docker Compose stoppen
make compose-down
```

### Skill Registry Server

```bash
# Server mit Standard-Config starten
./bin/skill-registry

# Mit custom Config
./bin/skill-registry -config /path/to/config.yaml

# Mit Environment Variables
SKILL_REGISTRY_ADDR=:9000 \
SKILL_REGISTRY_DATA_DIR=/data \
./bin/skill-registry
```

### skforge CLI Befehle

```bash
# Login konfigurieren (Username/Password)
skforge login
# Registry URL: http://localhost:8080
# Username: admin
# Password: ****

# Version anzeigen
skforge version

# Skill validieren (vor dem Upload)
skforge validate ./my-skill/

# Skill publishen (benötigt write scope)
skforge publish ./my-skill/ \
  --namespace myteam \
  --name my-skill \
  --version 1.0.0 \
  --registry http://localhost:8080

# Skills suchen (öffentlich, kein Token benötigt)
skforge search "keyword"

# Skill-Info abrufen (öffentlich, kein Token benötigt)
skforge info namespace/skill-name

# Skill installieren (öffentlich, kein Token benötigt)
skforge install namespace/skill-name@version

# Skill löschen (benötigt delete scope)
skforge delete namespace/skill-name@version

# Token Management
skforge token create --name "CI Token" --scopes write
skforge token list
skforge token revoke <token-id>

# Hilfe anzeigen
skforge --help
```

### Entwicklungs-Workflow

**Lokale Entwicklung mit Live-Reload:**
```bash
# Terminal 1: Backend starten
make build && ./bin/skill-registry

# Terminal 2: Frontend Dev-Server (mit Hot-Reload)
cd web && npm run dev
# Öffne http://localhost:3000
```

**Produktions-Build testen:**
```bash
# Alles bauen
make build-all

# Server starten (serviert Web-UI automatisch)
./bin/skill-registry

# Öffne http://localhost:8080
```

## Configuration

Configuration can be provided via `config.yaml` or environment variables.

### Example Configuration

```yaml
server:
  addr: ":8080"

storage:
  data_dir: "./data"
  max_package_size_mb: 50
  allowed_package_types:
    - tgz
    - zip

database:
  path: "./data/registry.db"

auth:
  enabled: true  # Set to true for production, false for development
  # Note: User authentication is database-backed.
  # Create initial admin user with:
  #   ./bin/skill-registry admin create-user --username admin --password <pass> --role admin

validation:
  blocked_extensions:
    - ".exe"
    - ".dll"
    - ".so"
    - ".dylib"
    - ".bin"
```

### Environment Variables

All configuration options can be overridden with environment variables:

- `SKILL_REGISTRY_ADDR` - Server address (default: `:8080`)
- `SKILL_REGISTRY_DATA_DIR` - Data directory (default: `./data`)
- `SKILL_REGISTRY_DB_PATH` - Database path (default: `./data/registry.db`)
- `SKILL_REGISTRY_MAX_PACKAGE_SIZE_MB` - Maximum package size
- `SKILL_REGISTRY_AUTH_ENABLED` - Enable authentication (default: `false`)
- `SKILL_REGISTRY_ADMIN_TOKEN` - Admin token value
- `SKILL_REGISTRY_READ_TOKEN` - Read-only token value

## API Usage

### Health Check

```bash
curl http://localhost:8080/healthz
```

### Authentication

#### Login (Get Token)

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin123"}'
```

Response:
```json
{
  "token": "skt_...",
  "user": "admin",
  "role": "admin",
  "scopes": ["read", "write", "delete"],
  "expires_at": "2026-07-04T21:30:23Z"
}
```

#### Create Token

```bash
curl -X POST http://localhost:8080/api/v1/tokens \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name":"CI Pipeline","scopes":["write"]}'
```

#### List Tokens

```bash
curl http://localhost:8080/api/v1/tokens \
  -H "Authorization: Bearer YOUR_TOKEN"
```

#### Revoke Token

```bash
curl -X DELETE http://localhost:8080/api/v1/tokens/1 \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### List Skills

```bash
curl http://localhost:8080/api/v1/skills
```

### Search Skills

```bash
curl "http://localhost:8080/api/v1/skills?q=documentation"
```

### Get Skill Details

```bash
curl http://localhost:8080/api/v1/skills/default/hello-skill
```

### Publish a Skill

```bash
curl -X PUT \
  -H "Content-Type: application/gzip" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  --data-binary @skill.tar.gz \
  http://localhost:8080/api/v1/skills/default/my-skill/versions/1.0.0
```

### Download a Skill

```bash
curl -o skill.tar.gz \
  http://localhost:8080/api/v1/skills/default/my-skill/versions/1.0.0/download
```

### Validate a Package

```bash
curl -X POST \
  -H "Content-Type: application/gzip" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  --data-binary @skill.tar.gz \
  http://localhost:8080/api/v1/validate
```

### Delete a Version

```bash
curl -X DELETE \
  -H "Authorization: Bearer YOUR_TOKEN" \
  http://localhost:8080/api/v1/skills/default/my-skill/versions/1.0.0
```

## Web UI

The registry includes a modern React web interface for browsing and managing skills, similar to Verdaccio's web UI.

### Features

- 🔍 Search and filter skills
- 📦 Browse skill catalog with statistics
- 📊 View detailed skill information
- 📋 List all versions of a skill
- ⬇️ Download skill packages
- 🎨 Modern purple gradient design

### Development

```bash
# Install web dependencies
cd web
npm install

# Start dev server (with API proxy to :8080)
npm run dev
# Web UI will be at http://localhost:3000
```

### Production Build

```bash
# Build web UI
make build-web

# Build everything (Go binaries + web UI)
make build-all

# Start the registry server
./bin/skill-registry

# Web UI will be served at http://localhost:8080
```

The production build places static files in `web/dist/`, which the Go server automatically serves when available.

## CLI Usage (`skforge`)

The `skforge` CLI tool provides a user-friendly interface to interact with the registry.

### Installation

```bash
# Build the CLI
make build

# The binary will be in ./bin/skforge
# Optionally, move it to your PATH
sudo mv ./bin/skforge /usr/local/bin/
```

### Login

```bash
skforge login
# Registry URL: http://localhost:8080
# Username: admin
# Password: ****
# ✅ Login successful!
```

The token is automatically saved to `~/.skforge/config.yaml`.

Or configure via environment variables:

```bash
export SKILL_REGISTRY_URL=http://localhost:8080
export SKILL_REGISTRY_TOKEN=your-token-here
```

### Publish a Skill

```bash
# Publish from a directory
skforge publish ./my-skill --registry http://localhost:8080

# Publish from an archive
skforge publish ./my-skill.tar.gz --registry http://localhost:8080
```

### Search Skills

```bash
skforge search documentation
```

### Get Skill Information

```bash
skforge info default/my-skill
```

### Install a Skill

```bash
# Install to default location (.agents/skills)
skforge install default/my-skill@1.0.0

# Install to custom location
skforge install default/my-skill@1.0.0 --target ./skills
```

### Validate a Package

```bash
# Validate from directory
skforge validate ./my-skill

# Validate from archive
skforge validate ./my-skill.tar.gz
```

## Skill Package Format

A valid skill package must contain:

1. **`SKILL.md`** (required) - The main skill definition file
2. Optional subdirectories:
   - `references/` - Reference materials
   - `scripts/` - Automation scripts
   - `assets/` - Images, diagrams, etc.

### Skill Metadata

Metadata can be provided in YAML frontmatter in `SKILL.md`:

```yaml
---
name: company.documentation-review
version: 1.0.0
description: Reviews documentation against internal standards.
tags:
  - documentation
  - review
  - quality
compatibility:
  codex: ">=0.1.0"
  claude: ">=1.0.0"
entrypoint: SKILL.md
license: internal
owners:
  - platform-team
---

# Your Skill Content Here
```

Alternatively, provide a separate `skill.yaml` or `skill.json` file.

### Example Skill Structure

```
my-skill/
├── SKILL.md
├── README.md
├── references/
│   └── api-docs.md
├── scripts/
│   └── validate.sh
└── assets/
    └── diagram.png
```

## 📚 Best Practices für Skills

### 1. Naming Conventions

**Skill Names:**
- Verwende lowercase (kleinbuchstaben)
- Erlaubt: Buchstaben, Zahlen, Punkt (`.`), Bindestrich (`-`), Unterstrich (`_`)
- Muss mit Buchstabe oder Zahl beginnen
- Beispiele: `my-skill`, `data.processor`, `code_review`

**Namespaces:**
- Nutze Namespaces für Organisation: `myteam/my-skill`
- Standard-Namespace ist `default`
- Firmen: `company-name/skill-name`
- Teams: `team-name/skill-name`

**Versioning:**
- Folge [Semantic Versioning](https://semver.org/): `MAJOR.MINOR.PATCH`
- Breaking Changes: MAJOR erhöhen (`1.0.0` → `2.0.0`)
- Neue Features: MINOR erhöhen (`1.0.0` → `1.1.0`)
- Bug-Fixes: PATCH erhöhen (`1.0.0` → `1.0.1`)

### 2. SKILL.md Struktur

Empfohlene Struktur für `SKILL.md`:

```markdown
---
name: my-awesome-skill
version: 1.0.0
description: Kurze, prägnante Beschreibung
tags:
  - category
  - feature-type
owners:
  - your-team
---

# My Awesome Skill

## Übersicht
Was macht dieser Skill? Wann sollte er verwendet werden?

## Verwendung
Klare Anweisungen zur Nutzung des Skills.

## Beispiele
Konkrete Beispiele mit Code oder Befehlen.

## Konfiguration
Welche Optionen gibt es?

## Referenzen
Links zu weiterführender Dokumentation.
```

### 3. Metadaten

**Wichtige Felder:**
- `description`: Kurz (max. 200 Zeichen), aussagekräftig
- `tags`: 3-5 relevante Tags für bessere Auffindbarkeit
- `owners`: Team oder Person, die verantwortlich ist
- `version`: Immer SemVer verwenden

**Tags richtig nutzen:**
- Kategorie: `frontend`, `backend`, `testing`, `documentation`
- Technologie: `react`, `python`, `typescript`, `docker`
- Zweck: `review`, `validation`, `generation`, `analysis`

### 4. Dokumentation

**README.md (optional, aber empfohlen):**
- Detaillierte Installationsanleitung
- Verwendungsbeispiele
- Troubleshooting
- Changelog

**references/ Verzeichnis:**
- API-Dokumentation
- Externe Links
- Zusätzliche Ressourcen

### 5. Package-Größe

- Halte Packages klein (< 10 MB wenn möglich)
- Entferne unnötige Dateien
- Nutze `.gitignore` Patterns
- Große Assets auslagern (URLs statt Dateien)

### 6. Testing vor Publish

```bash
# Immer vor dem Upload validieren
skforge validate ./my-skill/

# Test-Installation durchführen
skforge publish ./my-skill/ --registry http://localhost:8080
skforge install default/my-skill@1.0.0 --target ./test-install
```

### 7. Versionierung Strategy

**Development:**
- Nutze Prerelease-Versionen: `1.0.0-alpha.1`, `1.0.0-beta.1`
- Teste gründlich vor stabilen Releases

**Production:**
- Publiziere stabile Versionen ohne Prerelease-Tags
- Dokumentiere Breaking Changes im README
- Erstelle einen Changelog

**Example Workflow:**
```bash
# Development
skforge publish . --version 2.0.0-alpha.1

# Testing
skforge publish . --version 2.0.0-beta.1

# Production Release
skforge publish . --version 2.0.0
```

### 8. Sicherheit

- **Keine Secrets** in Skills committen
- **Keine Binaries** außer absolut nötig
- Nutze `references/` für externe Links statt große Downloads
- Überprüfe Dependencies regelmäßig

### 9. Update-Strategie

**Vor dem Update:**
```bash
# Aktuelle Version abrufen
skforge info myteam/my-skill

# Neue Version vorbereiten
# Version in SKILL.md erhöhen
vim SKILL.md

# Validieren
skforge validate .
```

**Update durchführen:**
```bash
# Neue Version publishen
skforge publish . --version 1.1.0

# Überprüfen
skforge info myteam/my-skill
```

## Security Model

### Package Validation

The registry validates all uploaded packages:

- ✅ Must contain `SKILL.md`
- ✅ `SKILL.md` must not be empty
- ✅ Skill name must match `^[a-z0-9][a-z0-9._-]{1,127}$`
- ✅ Version must be valid SemVer
- ✅ No absolute paths allowed
- ✅ No path traversal (`..`) allowed
- ✅ No symlinks escaping root
- ✅ Package size limits enforced
- ✅ Blocked file extensions (`.exe`, `.dll`, `.so`, etc.)
- ✅ Content integrity with SHA-256 digests

### Authentication

Die Registry unterstützt **User-basierte Authentifizierung** mit Token-Management:

#### 🔐 Setup (Initial Admin User)

```bash
# 1. Admin-User erstellen (server-seitig)
docker compose exec registry /app/skill-registry admin create-user \
  --username admin \
  --password <sicheres-passwort> \
  --role admin

# Oder lokal (ohne Docker)
./bin/skill-registry admin create-user \
  --username admin \
  --password admin123 \
  --role admin \
  --db ./data/registry.db
```

#### 👤 Login (Client)

```bash
# Via CLI
skforge login
# Registry URL: http://localhost:8080
# Username: admin
# Password: ****
# ✅ Login successful!
#    User: admin
#    Role: admin
#    Scopes: read, write, delete
```

Der Login-Token wird automatisch in `~/.skforge/config.yaml` gespeichert.

#### 🎫 Token-Management

**Token erstellen:**
```bash
# CI/CD Token (nur write)
skforge token create --name "CI Pipeline" --scopes write

# Developer Token (read + write)
skforge token create --name "Dev Token" --scopes read,write

# Admin Token (alle Rechte)
skforge token create --name "Admin Token" --scopes read,write,delete

# ✅ Token created successfully!
#    Name: CI Pipeline
#    Scopes: write
#    Token: skt_abc123...
#    
# ⚠️  Save this token - it will only be shown once!
```

**Tokens auflisten:**
```bash
skforge token list
# Tokens:
# 
#   ID: 1
#   Name: CI Pipeline
#   Scopes: write
#   Status: active
#   Created: 2026-06-04T12:00:00Z
# 
#   ID: 2
#   Name: Dev Token
#   Scopes: read, write
#   Status: active
#   Created: 2026-06-04T13:00:00Z
```

**Token widerrufen:**
```bash
skforge token revoke 1
# ⚠️  Revoke token #1? (y/N): y
# ✅ Token #1 revoked
```

#### 🔒 Scopes & Permissions

| Scope | Berechtigung | Beispiel |
|-------|-------------|---------|
| `read` | Skills durchsuchen und herunterladen | `skforge search`, `skforge install` |
| `write` | Skills hochladen | `skforge publish` |
| `delete` | Skills löschen | `skforge delete` |

**Rollen:**
- **`user`**: Kann `read` und `write` Tokens erstellen
- **`admin`**: Kann alle Tokens erstellen (inkl. `delete`)

#### 🌐 Öffentlicher Zugriff

**Download/Search ist immer öffentlich** (ohne Token):
```bash
# Funktioniert ohne Login
curl http://localhost:8080/api/v1/skills
skforge search
skforge install default/skill@1.0.0
```

**Publish/Delete erfordern Token:**
```bash
# Benötigt Token mit write scope
skforge publish ./my-skill

# Benötigt Token mit delete scope
skforge delete default/skill@1.0.0
```

#### 🔧 Auth aktivieren/deaktivieren

```yaml
# config.yaml
auth:
  enabled: true  # false = keine Auth (Entwicklung)
```

```yaml
# docker-compose.yml
environment:
  - SKILL_REGISTRY_AUTH_ENABLED=true
```

### Audit Logging

All write operations (publish, delete) are logged with:
- Action type
- Namespace/name/version
- Actor (token name or "anonymous")
- Timestamp
- Success/failure status

## Development

### Prerequisites

- Go 1.25 or higher
- SQLite 3
- Node.js 20+ (for Web UI development)
- Make (optional, for convenience)

### Building

```bash
# Download dependencies
make deps

# Run tests
make test

# Run linter
make lint

# Build binaries
make build
```

### Testing

```bash
# Run all tests
make test

# Run tests with coverage
go test -v -race -coverprofile=coverage.txt ./...

# View coverage
go tool cover -html=coverage.txt
```

### Project Structure

```
skill-registry/
├── cmd/
│   ├── skill-registry/    # Main registry server
│   └── skforge/           # CLI tool
├── internal/
│   ├── api/               # HTTP handlers
│   ├── auth/              # Authentication
│   ├── config/            # Configuration management
│   ├── metadata/          # SQLite models and repository
│   ├── registry/          # Core registry logic
│   ├── storage/           # Blob storage
│   ├── validation/        # Package validation
│   └── observability/     # Logging and middleware
├── pkg/
│   └── client/            # Go client library
├── web/                   # React web UI
│   ├── src/               # React components and pages
│   ├── public/            # Static assets
│   └── dist/              # Built web UI
├── migrations/            # Database migrations
├── openapi/               # OpenAPI specification
├── examples/              # Example skills
├── tests/                 # Integration tests
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── README.md
```

## Architecture

### Storage Layout

```
data/
├── blobs/
│   └── sha256/
│       └── ab/
│           └── cd/
│               └── abcdef123456...
├── packages/
│   └── namespace/
│       └── skillname/
│           └── 1.0.0.tgz
└── registry.db
```

- **Blobs**: Content-addressable storage using SHA-256 digests
- **Packages**: Symbolic links or copies organized by namespace/name/version
- **Database**: SQLite for metadata, versions, and audit logs

### Components

1. **HTTP API**: Chi router with structured logging and request IDs
2. **Validator**: Security-focused package validation
3. **Storage**: Filesystem-based with content addressing
4. **Metadata Repository**: SQLite with migration support
5. **Registry**: Orchestrates validation, storage, and metadata
6. **Authenticator**: Token-based auth with scope checking
7. **CLI Client**: User-friendly interface with config management

## Known Limitations

### Current Version (v1.0.0)

- ✅ Basic upstream proxy interface defined, but not fully implemented
- ✅ SemVer range resolution not yet supported (only exact versions and "latest")
- ✅ Hard delete requires manual database operations
- ✅ CLI metadata extraction is simplified (doesn't parse SKILL.md frontmatter)
- ✅ No built-in replication or high availability
- ✅ Package signing is tracked but not enforced

### Performance Considerations

- SQLite is suitable for small to medium deployments
- For large-scale deployments, consider:
  - Adding a caching layer (Redis)
  - Using object storage (S3) for blobs
  - Implementing PostgreSQL backend
  - Load balancing with shared storage

## 🔧 Troubleshooting

### Server startet nicht

**Problem:** `address already in use`
```bash
# Port ist bereits belegt, anderen Port verwenden
SKILL_REGISTRY_ADDR=:9000 ./bin/skill-registry
```

**Problem:** `cannot open database file`
```bash
# Datenverzeichnis existiert nicht
mkdir -p ./data
./bin/skill-registry
```

### Web-UI zeigt keine Skills

**Problem:** API gibt 404 zurück
```bash
# Überprüfen, ob Server läuft
curl http://localhost:8080/healthz

# API direkt testen
curl http://localhost:8080/api/v1/skills
```

**Problem:** Web-UI wird nicht geladen
```bash
# Web-UI muss erst gebaut werden
make build-web

# Dann Server neu starten
./bin/skill-registry

# Oder im Dev-Modus
cd web && npm run dev
```

### Skill-Upload schlägt fehl

**Problem:** `SKILL.md not found`
```bash
# Jeder Skill braucht eine SKILL.md Datei
touch SKILL.md
echo "# My Skill" > SKILL.md
```

**Problem:** `package too large`
```bash
# Max. Package-Größe in config.yaml erhöhen
storage:
  max_package_size_mb: 100
```

**Problem:** `authentication required`
```bash
# Token konfigurieren
export SKILL_REGISTRY_TOKEN=your-token
skforge publish ./my-skill

# Oder Auth in config.yaml deaktivieren
auth:
  enabled: false
```

### CLI-Probleme

**Problem:** `skforge: command not found`
```bash
# CLI ist nicht im PATH
./bin/skforge --help

# Oder in PATH verschieben
sudo mv ./bin/skforge /usr/local/bin/
```

**Problem:** Config-Fehler
```bash
# Config zurücksetzen
rm -rf ~/.skforge
skforge login
```

### Entwicklung

**Problem:** Web-Build schlägt fehl
```bash
# Node-Version überprüfen (>=18 erforderlich)
node --version

# Dependencies neu installieren
cd web
rm -rf node_modules package-lock.json
npm install
```

**Problem:** Tests schlagen fehl
```bash
# CGO muss für SQLite aktiviert sein
CGO_ENABLED=1 go test ./...

# Oder über Makefile
make test
```

### Logs und Debugging

```bash
# Server mit Debug-Logs starten
LOG_LEVEL=debug ./bin/skill-registry

# API direkt testen
curl -v http://localhost:8080/api/v1/skills

# Datenbank inspizieren
sqlite3 ./data/registry.db "SELECT * FROM skills;"
```

## Roadmap

### Phase 2 (Next Release)

- [ ] Full upstream proxy implementation with caching
- [ ] SemVer range resolution (`^1.0.0`, `~1.2.3`)
- [x] Web UI for browsing and searching skills ✅ **Implemented**
- [ ] Package signing and verification
- [ ] Metrics endpoint with Prometheus format
- [ ] PostgreSQL backend option
- [ ] S3-compatible storage backend

### Phase 3 (Future)

- [ ] Multi-registry federation
- [ ] Access control lists (ACLs) for namespaces
- [ ] Webhooks for publish/delete events
- [ ] Package deprecation workflow
- [ ] Vulnerability scanning integration
- [ ] GraphQL API
- [ ] Skill dependencies and resolution

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass (`make test`)
5. Run the linter (`make lint`)
6. Submit a pull request

## License

MIT License - see LICENSE file for details

## Support

- **Issues**: [GitHub Issues](https://github.com/skillforge/skill-registry/issues)
- **Documentation**: [Full API Documentation](./openapi/openapi.yaml)
- **Community**: Join our discussions

## Acknowledgments

Inspired by:
- [Verdaccio](https://verdaccio.org/) - Private npm registry
- [Docker Registry](https://docs.docker.com/registry/) - Container image registry
- [Helm Charts](https://helm.sh/) - Package management for Kubernetes

---

**Built with ❤️ for the AI Agent community**
