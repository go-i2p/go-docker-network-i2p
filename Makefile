# Go I2P Docker Network Plugin

.PHONY: all build test clean install uninstall fmt vet lint help docker-build docker-push release

# Build variables
BINARY_NAME := i2p-network-plugin
PKG := github.com/go-i2p/go-docker-network-i2p
CMD_DIR := ./cmd/$(BINARY_NAME)
BUILD_DIR := ./bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT) -s -w"

# Go variables
GO := go
GOFMT := gofmt
GOVET := $(GO) vet
GOLINT := golangci-lint
GOTEST := $(GO) test

# Docker plugin variables
PLUGIN_NAME := go-i2p/network-i2p
PLUGIN_TAG := latest
PLUGIN_DIR := ./plugin

# Docker image variables
DOCKER_IMAGE := golovers/i2p-network-plugin
DOCKER_TAG := $(VERSION)
DOCKER_LATEST := $(DOCKER_IMAGE):latest

all: fmt vet test build ## Run format, vet, test and build

build: ## Build the plugin binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...

test-coverage: test ## Run tests and show coverage
	$(GO) tool cover -html=coverage.out

fmt: ## Format code
	@echo "Formatting code..."
	$(GOFMT) -l -w .

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

lint: ## Run golangci-lint
	@echo "Running golangci-lint..."
	$(GOLINT) run

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

install: build ## Install the plugin locally
	@echo "Installing plugin..."
	sudo mkdir -p /run/docker/plugins
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

uninstall: ## Uninstall the plugin
	@echo "Uninstalling plugin..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)

deps: ## Install dependencies
	@echo "Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy

deps-dev: ## Install development dependencies
	@echo "Installing development dependencies..."
	@which golangci-lint > /dev/null || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin

plugin-package: build ## Package as Docker plugin
	@echo "Creating plugin package..."
	@mkdir -p $(PLUGIN_DIR)
	cp $(BUILD_DIR)/$(BINARY_NAME) $(PLUGIN_DIR)/
	cp plugin.json $(PLUGIN_DIR)/
	docker plugin create $(PLUGIN_NAME):$(PLUGIN_TAG) $(PLUGIN_DIR)

plugin-install: plugin-package ## Install Docker plugin
	docker plugin install $(PLUGIN_NAME):$(PLUGIN_TAG)

plugin-uninstall: ## Uninstall Docker plugin
	docker plugin disable $(PLUGIN_NAME):$(PLUGIN_TAG) || true
	docker plugin rm $(PLUGIN_NAME):$(PLUGIN_TAG) || true

##@ Docker Image Targets

docker-build: ## Build Docker image for the plugin
	@echo "Building Docker image $(DOCKER_IMAGE):$(DOCKER_TAG)..."
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		--build-arg GIT_COMMIT=$(GIT_COMMIT) \
		-t $(DOCKER_IMAGE):$(DOCKER_TAG) \
		-t $(DOCKER_LATEST) \
		.
	@echo "Docker image built successfully"

docker-push: docker-build ## Push Docker image to registry
	@echo "Pushing Docker image to registry..."
	docker push $(DOCKER_IMAGE):$(DOCKER_TAG)
	docker push $(DOCKER_LATEST)
	@echo "Docker image pushed successfully"

docker-run: docker-build ## Run the plugin in a Docker container
	@echo "Running plugin in Docker container..."
	docker run -d \
		--name i2p-network-plugin \
		--privileged \
		--network host \
		-v /run/docker/plugins:/run/docker/plugins \
		-v /var/lib/i2p-network-plugin:/var/lib/i2p-network-plugin \
		$(DOCKER_IMAGE):$(DOCKER_TAG)

docker-stop: ## Stop and remove plugin container
	@echo "Stopping plugin container..."
	docker stop i2p-network-plugin || true
	docker rm i2p-network-plugin || true

docker-logs: ## View plugin container logs
	docker logs -f i2p-network-plugin

##@ Release Targets

release-artifacts: build ## Create release artifacts (binary + checksums)
	@echo "Creating release artifacts for version $(VERSION)..."
	@mkdir -p dist
	@cp $(BUILD_DIR)/$(BINARY_NAME) dist/$(BINARY_NAME)-$(VERSION)-linux-amd64
	@cd dist && sha256sum $(BINARY_NAME)-$(VERSION)-linux-amd64 > $(BINARY_NAME)-$(VERSION)-linux-amd64.sha256
	@cd dist && tar czf $(BINARY_NAME)-$(VERSION)-linux-amd64.tar.gz $(BINARY_NAME)-$(VERSION)-linux-amd64 $(BINARY_NAME)-$(VERSION)-linux-amd64.sha256
	@echo "Release artifacts created in dist/"
	@ls -lh dist/

release-tag: ## Create and push a git tag for release
	@if [ "$(VERSION)" = "dev" ]; then \
		echo "Error: Cannot create release from dev version. Commit changes and create a tag."; \
		exit 1; \
	fi
	@echo "Creating release tag $(VERSION)..."
	git tag -a $(VERSION) -m "Release $(VERSION)"
	git push origin $(VERSION)
	@echo "Tag $(VERSION) created and pushed"

release: test release-artifacts docker-build ## Full release process (test, artifacts, docker)
	@echo ""
	@echo "=========================================="
	@echo "Release $(VERSION) prepared successfully!"
	@echo "=========================================="
	@echo ""
	@echo "Artifacts created:"
	@ls -lh dist/
	@echo ""
	@echo "Docker image: $(DOCKER_IMAGE):$(DOCKER_TAG)"
	@echo ""
	@echo "Next steps:"
	@echo "  1. Review the artifacts in dist/"
	@echo "  2. Push Docker image: make docker-push"
	@echo "  3. Create GitHub release and upload artifacts"
	@echo "  4. Update documentation with new version"
	@echo ""

##@ Installation Targets

system-install: build ## Install plugin system-wide using install script
	@echo "Installing plugin system-wide..."
	@if [ ! -f scripts/install.sh ]; then \
		echo "Error: scripts/install.sh not found"; \
		exit 1; \
	fi
	sudo bash scripts/install.sh

system-uninstall: ## Uninstall plugin system-wide using install script
	@echo "Uninstalling plugin..."
	sudo bash scripts/install.sh --uninstall

help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*##"; printf "\033[36m\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)