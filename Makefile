# Ansible Go Makefile
# Copyright (c) 2024 Ansible Project

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOLINT=golangci-lint

# Build parameters
BINARY_NAME=ansible
BINARY_UNIX=$(BINARY_NAME)_unix
VERSION=2.19.0-go
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GO_VERSION=1.24
LDFLAGS=-ldflags="-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME) -X main.gitCommit=$(GIT_COMMIT)"

# Directories
BUILD_DIR=build
DIST_DIR=dist
CMD_DIR=cmd
PKG_DIR=pkg
INTERNAL_DIR=internal

# Binary names (only include implemented binaries)
ANSIBLE_BINARIES=ansible

# Platforms for cross-compilation
PLATFORMS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build clean test test-verbose coverage lint format deps check help install uninstall

# Default target
all: deps lint test build

# Help target
help:
	@echo "Available targets:"
	@echo "  all          - Run deps, lint, test, and build"
	@echo "  build        - Build all binaries for current platform"
	@echo "  build-linux  - Build all binaries for Linux"
	@echo "  cross-build  - Build for all platforms"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run all tests"
	@echo "  test-verbose - Run tests with verbose output"
	@echo "  coverage     - Run tests with coverage report"
	@echo "  lint         - Run linter"
	@echo "  format       - Format code"
	@echo "  deps         - Download dependencies"
	@echo "  check        - Run all checks (lint, test, etc.)"
	@echo "  install      - Install binaries to GOBIN"
	@echo "  uninstall    - Remove binaries from GOBIN"
	@echo "  docker       - Build Docker image"
	@echo "  release      - Create release packages"

# Build all binaries for current platform
build: deps
	@echo "Building for current platform..."
	@mkdir -p $(BUILD_DIR)
	@for binary in $(ANSIBLE_BINARIES); do \
		echo "Building $$binary..."; \
		$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$$binary ./$(CMD_DIR)/$$binary || exit 1; \
	done

# Build for Linux (test platform)
build-linux: deps
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)/linux
	@for binary in $(ANSIBLE_BINARIES); do \
		echo "Building $$binary for Linux..."; \
		GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/linux/$$binary ./$(CMD_DIR)/$$binary || exit 1; \
	done

# Cross-platform build
cross-build: deps
	@echo "Building for all platforms..."
	@mkdir -p $(DIST_DIR)
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		echo "Building for $$os/$$arch..."; \
		mkdir -p $(DIST_DIR)/$$os-$$arch; \
		for binary in $(ANSIBLE_BINARIES); do \
			output=$(DIST_DIR)/$$os-$$arch/$$binary; \
			if [ "$$os" = "windows" ]; then output=$$output.exe; fi; \
			GOOS=$$os GOARCH=$$arch $(GOBUILD) $(LDFLAGS) -o $$output ./$(CMD_DIR)/$$binary || exit 1; \
		done; \
		cd $(DIST_DIR) && tar -czf ansible-go-$(VERSION)-$$os-$$arch.tar.gz $$os-$$arch/ && cd ..; \
	done

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -rf $(DIST_DIR)
	rm -f coverage.out

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Run tests
test: deps
	$(GOTEST) -v ./...

# Run tests with verbose output
test-verbose: deps
	$(GOTEST) -v -race -count=1 ./...

# Run tests with coverage
coverage: deps
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run linter
lint: deps
	$(GOLINT) run --timeout=5m ./...

# Format code
format:
	$(GOCMD) fmt ./...
	goimports -w .

# Run all checks
check: lint test
	@echo "All checks passed!"

# Install binaries
install: build
	@echo "Installing binaries to $(GOBIN)..."
	@for binary in $(ANSIBLE_BINARIES); do \
		echo "Installing $$binary..."; \
		cp $(BUILD_DIR)/$$binary $(GOBIN)/$$binary; \
	done

# Uninstall binaries
uninstall:
	@echo "Removing binaries from $(GOBIN)..."
	@for binary in $(ANSIBLE_BINARIES); do \
		echo "Removing $$binary..."; \
		rm -f $(GOBIN)/$$binary; \
	done

# Docker build
docker:
	docker build -t ansible-go:$(VERSION) .
	docker tag ansible-go:$(VERSION) ansible-go:latest

# Create release packages
release: cross-build
	@echo "Creating release packages..."
	@cd $(DIST_DIR) && \
	for file in *.tar.gz; do \
		echo "Creating checksum for $$file..."; \
		sha256sum $$file > $$file.sha256; \
	done
	@echo "Release packages created in $(DIST_DIR)/"

# Development targets

# Run with race detection
dev-test:
	$(GOTEST) -race -v ./...

# Run specific test
test-pkg:
	@if [ -z "$(PKG)" ]; then echo "Usage: make test-pkg PKG=package_name"; exit 1; fi
	$(GOTEST) -v ./$(PKG_DIR)/$(PKG)/...

# Run benchmarks
bench:
	$(GOTEST) -bench=. -benchmem ./...

# Generate mocks (if using mockgen)
generate-mocks:
	$(GOCMD) generate ./...

# Update dependencies
update-deps:
	$(GOMOD) get -u ./...
	$(GOMOD) tidy

# Vendor dependencies
vendor:
	$(GOMOD) vendor

# Static analysis
static-analysis: lint
	govulncheck ./...
	gosec ./...

# Performance profiling
profile-cpu:
	$(GOTEST) -cpuprofile=cpu.prof -bench=. ./...

profile-mem:
	$(GOTEST) -memprofile=mem.prof -bench=. ./...

# Documentation
docs:
	godoc -http=:6060

# Check for security vulnerabilities
security:
	govulncheck ./...

# Pre-commit hooks
pre-commit: format lint test
	@echo "Pre-commit checks passed!"

# CI/CD targets
ci-test: deps lint test coverage

ci-build: deps build-linux

# Quick build for development
quick-build:
	$(GOBUILD) -o $(BUILD_DIR)/ansible ./$(CMD_DIR)/ansible

# Run the main ansible binary
run: quick-build
	./$(BUILD_DIR)/ansible $(ARGS)

# Development server
dev-server: quick-build
	./$(BUILD_DIR)/ansible --server --host localhost --port 8443

# Show build info
info:
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Go Version: $(shell $(GOCMD) version)"

# Check if all required tools are installed
check-tools:
	@command -v $(GOCMD) >/dev/null 2>&1 || { echo "Go is required but not installed."; exit 1; }
	@command -v $(GOLINT) >/dev/null 2>&1 || { echo "golangci-lint is required but not installed."; exit 1; }
	@command -v git >/dev/null 2>&1 || { echo "git is required but not installed."; exit 1; }
	@echo "Checking Go version..."
	@go_version=$$($(GOCMD) version | grep -o 'go[0-9]\+\.[0-9]\+' | head -1 | cut -d'o' -f2); \
	required_version="1.24"; \
	if [ "$$(printf '%s\n' "$$required_version" "$$go_version" | sort -V | head -n1)" = "$$required_version" ]; then \
		echo "Go version $$go_version meets minimum requirement ($(GO_VERSION))"; \
	else \
		echo "Go version $$go_version is below minimum requirement ($(GO_VERSION))"; \
		exit 1; \
	fi
	@echo "All required tools are installed and meet version requirements."