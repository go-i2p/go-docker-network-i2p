# Go I2P Docker Network Plugin

.PHONY: all build test clean install uninstall fmt vet lint help

# Build variables
BINARY_NAME := i2p-network-plugin
PKG := github.com/go-i2p/go-docker-network-i2p
CMD_DIR := ./cmd/$(BINARY_NAME)
BUILD_DIR := ./bin
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -s -w"

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

help: ## Show this help message
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*##"; printf "\033[36m\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)