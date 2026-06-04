.PHONY: build test lint run docker-build compose-up compose-down clean build-web install-web dev-web

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOVET=$(GOCMD) vet
GOFMT=gofmt

# Binary names
REGISTRY_BINARY=skill-registry
CLI_BINARY=skforge

# Build directory
BIN_DIR=./bin
WEB_DIR=./web

all: build

build:
	@echo "Building binaries..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(BIN_DIR)/$(REGISTRY_BINARY) ./cmd/skill-registry
	$(GOBUILD) -o $(BIN_DIR)/$(CLI_BINARY) ./cmd/skforge
	@echo "Build complete!"

test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.txt -covermode=atomic ./...
	@echo "Tests complete!"

lint:
	@echo "Running linters..."
	$(GOVET) ./...
	@echo "Checking formatting..."
	@test -z "$$($(GOFMT) -l .)" || ($(GOFMT) -l . && exit 1)
	@echo "Lint complete!"

run: build
	@echo "Starting skill registry..."
	$(BIN_DIR)/$(REGISTRY_BINARY) -config config.yaml

docker-build:
	@echo "Building Docker image..."
	docker build -t skill-registry:latest .
	@echo "Docker build complete!"

compose-up:
	@echo "Starting with docker-compose..."
	docker-compose up -d
	@echo "Services started!"

compose-down:
	@echo "Stopping docker-compose services..."
	docker-compose down
	@echo "Services stopped!"

clean:
	@echo "Cleaning up..."
	rm -rf $(BIN_DIR)
	rm -rf ./data
	rm -rf $(WEB_DIR)/dist
	rm -rf $(WEB_DIR)/node_modules
	rm -f coverage.txt
	@echo "Clean complete!"

# Web UI targets
install-web:
	@echo "Installing web UI dependencies..."
	cd $(WEB_DIR) && npm install
	@echo "Web dependencies installed!"

build-web: install-web
	@echo "Building web UI..."
	cd $(WEB_DIR) && npm run build
	@echo "Web UI built!"

dev-web:
	@echo "Starting web UI dev server..."
	cd $(WEB_DIR) && npm run dev

# Full build including web UI
build-all: build build-web
	@echo "Full build complete!"
	rm -f coverage.txt
	@echo "Clean complete!"

install-tools:
	@echo "Installing development tools..."
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "Tools installed!"

deps:
	@echo "Downloading dependencies..."
	$(GOCMD) mod download
	@echo "Dependencies downloaded!"

tidy:
	@echo "Tidying module..."
	$(GOCMD) mod tidy
	@echo "Tidy complete!"
