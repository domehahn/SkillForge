# Build Web UI
FROM node:22-alpine AS web-builder

WORKDIR /web
COPY SkillForge/skill-registry/web/package*.json ./
RUN npm ci
COPY SkillForge/skill-registry/web/ ./
RUN npm run build

# Build Go binaries
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git make gcc musl-dev sqlite-dev

WORKDIR /build

# Copy sklib source next to skill-registry so the go.mod replace directive resolves.
COPY sklib/ ./sklib/

# Copy skill-registry module manifests and download external dependencies.
# GOWORK=off disables workspace mode; go.mod replace handles sklib locally.
COPY SkillForge/skill-registry/go.mod SkillForge/skill-registry/go.sum ./skill-registry/
RUN cd skill-registry && GOWORK=off go mod download

# Copy full skill-registry source
COPY SkillForge/skill-registry/ ./skill-registry/

WORKDIR /build/skill-registry

RUN GOWORK=off go mod tidy
RUN CGO_ENABLED=1 GOWORK=off go build -o /app/skill-registry ./cmd/skill-registry
RUN CGO_ENABLED=1 GOWORK=off go build -o /app/skforge ./cmd/skforge

# Runtime image
FROM alpine:3.21

RUN apk add --no-cache ca-certificates sqlite-libs \
    && adduser -D -u 1001 -H -h /app registry

WORKDIR /app

COPY --from=builder /app/skill-registry /app/
COPY --from=builder /app/skforge /app/
COPY --from=web-builder /web/dist /app/web/dist

RUN mkdir -p /app/data /app/certs && chown -R registry:registry /app

USER registry

EXPOSE 8080 8443

ENV SKILL_REGISTRY_ADDR=":8080"
ENV SKILL_REGISTRY_DATA_DIR="/app/data"
ENV SKILL_REGISTRY_DB_PATH="/app/data/registry.db"
# TLS: mount cert/key into /app/certs and set these env vars to enable HTTPS
ENV SKILL_REGISTRY_TLS_ENABLED="false"
ENV SKILL_REGISTRY_TLS_CERT_FILE="/app/certs/tls.crt"
ENV SKILL_REGISTRY_TLS_KEY_FILE="/app/certs/tls.key"

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

CMD ["/app/skill-registry"]
