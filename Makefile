# Makefile for pixie-cli
# A CLI tool for generating Go backend projects
#
# Usage:
#   make dev              - Run CLI in development mode
#   make dev ARGS="..."   - Run CLI with arguments
#   make build            - Build the binary
#   make install          - Install to ~/.pixie/
#   make test             - Run tests
#   make lint             - Run linter (requires golangci-lint)
#   make fmt              - Format code
#   make clean            - Remove build artifacts
#   make help             - Show this help

.PHONY: help dev build install test lint fmt clean tidy check

# ─────────────────────────────────────────────────────────────────
# Configuration
# ─────────────────────────────────────────────────────────────────

# Binary configuration
BINARY_NAME := pixie
INSTALL_DIR := $(HOME)/.pixie

# Build configuration
GOARCH ?= $(shell go env GOARCH)
GOOS ?= $(shell go env GOOS)

# Version information (from git)
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Package paths
MODULE := $(shell go list -m)
VERSION_PKG := $(MODULE)/internal/version
MAIN_PACKAGE := ./cmd/cli/cli_core

# Build flags
LDFLAGS := -s -w \
	-X '$(VERSION_PKG).Version=$(VERSION)' \
	-X '$(VERSION_PKG).Commit=$(COMMIT)'

# Output directories
BIN_DIR := bin

# Arguments passthrough (usage: make dev ARGS="bootstrap --help")
ARGS ?=

# ─────────────────────────────────────────────────────────────────
# Default target
# ─────────────────────────────────────────────────────────────────

help: ## Show this help message
	@echo "pixie-cli - Backend Project Generator"
	@echo ""
	@echo "Usage: make [target] [ARGS=\"...\"]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ { printf "  %-15s %s\n", $$1, $$2 }' $(MAKEFILE_LIST)
	@echo ""
	@echo "Examples:"
	@echo "  make dev                         # Run CLI in dev mode"
	@echo "  make dev ARGS=\"bootstrap --help\" # Run with arguments"
	@echo "  make build                       # Build binary to ./bin/"
	@echo "  make install                     # Install to ~/.pixie/"
	@echo ""
	@echo "Version: $(VERSION) ($(COMMIT))"

# ─────────────────────────────────────────────────────────────────
# Development
# ─────────────────────────────────────────────────────────────────

dev: ## Run CLI in development mode (use ARGS="..." for arguments)
	@go run -ldflags="$(LDFLAGS)" $(MAIN_PACKAGE) $(ARGS)

# ─────────────────────────────────────────────────────────────────
# Build
# ─────────────────────────────────────────────────────────────────

build: ## Build the binary to ./bin/
	@echo "Building $(BINARY_NAME) $(VERSION) ($(COMMIT))..."
	@mkdir -p $(BIN_DIR)
	@go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Binary: $(BIN_DIR)/$(BINARY_NAME)"

build-all: ## Build for all supported platforms
	@echo "Building for all platforms..."
	@mkdir -p $(BIN_DIR)
	@GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)
	@GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)
	@GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	@GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)
	@echo "Built binaries in $(BIN_DIR)/"
	@ls -la $(BIN_DIR)/

# ─────────────────────────────────────────────────────────────────
# Install
# ─────────────────────────────────────────────────────────────────

install: ## Install to ~/.pixie/ directory
	@echo "Building $(BINARY_NAME)..."
	@go build -ldflags="$(LDFLAGS)" -o /tmp/$(BINARY_NAME) $(MAIN_PACKAGE)
	@echo "Installing to $(INSTALL_DIR)/..."
	@mkdir -p $(INSTALL_DIR)
	@mv /tmp/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@chmod +x $(INSTALL_DIR)/$(BINARY_NAME)
	@echo ""
	@echo "Installed: $(INSTALL_DIR)/$(BINARY_NAME)"
	@echo ""
	@echo "To add pixie to your PATH, add this line to your shell profile:"
	@echo "  export PATH=\"\$$PATH:$(INSTALL_DIR)\""
	@echo ""
	@echo "Then run: source ~/.bashrc  (or ~/.zshrc)"

uninstall: ## Remove installed binary
	@echo "Removing $(INSTALL_DIR)/$(BINARY_NAME)..."
	@rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Uninstalled."

# ─────────────────────────────────────────────────────────────────
# Testing
# ─────────────────────────────────────────────────────────────────

test: ## Run tests with coverage
	@echo "Running tests..."
	@go test ./... -v -cover

test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	@go test ./... -v -race

test-coverage: ## Run tests and generate coverage report
	@echo "Running tests with coverage report..."
	@go test ./... -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# ─────────────────────────────────────────────────────────────────
# Code Quality
# ─────────────────────────────────────────────────────────────────

fmt: ## Format code
	@echo "Formatting code..."
	@go fmt ./...

lint: ## Run linter (requires golangci-lint)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "Running linter..."; \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install with:"; \
		echo "  brew install golangci-lint"; \
		echo "  or: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

tidy: ## Tidy go modules
	@echo "Tidying modules..."
	@go mod tidy

vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

check: fmt vet lint test ## Run all checks (fmt, vet, lint, test)
	@echo "All checks passed!"

# ─────────────────────────────────────────────────────────────────
# Cleanup
# ─────────────────────────────────────────────────────────────────

clean: ## Remove build artifacts
	@echo "Cleaning..."
	@rm -rf $(BIN_DIR)
	@rm -f coverage.out coverage.html
	@echo "Clean complete."

# ─────────────────────────────────────────────────────────────────
# Information
# ─────────────────────────────────────────────────────────────────

version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Commit:  $(COMMIT)"
	@echo "Module:  $(MODULE)"
	@echo "Go:      $(shell go version)"

env: ## Show build environment
	@echo "GOOS:        $(GOOS)"
	@echo "GOARCH:      $(GOARCH)"
	@echo "VERSION:     $(VERSION)"
	@echo "COMMIT:      $(COMMIT)"
	@echo "MODULE:      $(MODULE)"
	@echo "MAIN:        $(MAIN_PACKAGE)"
	@echo "BIN_DIR:     $(BIN_DIR)"
	@echo "INSTALL_DIR: $(INSTALL_DIR)"
