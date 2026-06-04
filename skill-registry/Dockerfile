FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev sqlite-dev

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build binaries
RUN CGO_ENABLED=1 go build -o skill-registry ./cmd/skill-registry
RUN CGO_ENABLED=1 go build -o skillctl ./cmd/skillctl

# Runtime image
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /build/skill-registry /app/
COPY --from=builder /build/skillctl /app/

# Create data directory
RUN mkdir -p /app/data

# Expose port
EXPOSE 8080

# Set environment variables
ENV SKILL_REGISTRY_ADDR=":8080"
ENV SKILL_REGISTRY_DATA_DIR="/app/data"
ENV SKILL_REGISTRY_DB_PATH="/app/data/registry.db"

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

CMD ["/app/skill-registry"]
