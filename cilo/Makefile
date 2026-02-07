# Cilo Makefile
# Usage: make build or make install

# Read version from VERSION file
VERSION := $(shell cat VERSION 2>/dev/null || echo "0.1.0")

# Build metadata
BUILD_DIR := $(shell pwd)
BIN_DIR := ../bin
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go build flags with version info
LDFLAGS := -ldflags "-X github.com/sharedco/cilo/cmd.version=$(VERSION) \
	-X github.com/sharedco/cilo/cmd.commit=$(GIT_COMMIT) \
	-X github.com/sharedco/cilo/cmd.date=$(BUILD_DATE)"

.PHONY: all build install clean version test bump-patch bump-minor bump-major

# Default target
all: build

# Build binary with version info
build:
	@echo "Building cilo $(VERSION)..."
	@mkdir -p $(BIN_DIR)
	go build $(LDFLAGS) -o $(BIN_DIR)/cilo main.go
	@echo "✓ Built $(BIN_DIR)/cilo"
	@echo "  Version: $(VERSION)"
	@echo "  Commit:  $(GIT_COMMIT)"
	@echo "  Date:    $(BUILD_DATE)"

# Build and bump version - usage: make build BUMP=patch (or minor, major)
# Or: make patch, make minor, make major
build-bump:
	@if [ -z "$(BUMP)" ]; then \
		echo "Usage: make build BUMP=patch|minor|major"; \
		exit 1; \
	fi
	@$(MAKE) bump-$(BUMP)
	@$(MAKE) build

# Bump patch version (0.1.0 -> 0.1.1)
bump-patch:
	@echo "Bumping patch version..."
	@current_version=$$(cat VERSION); \
	major=$$(echo $$current_version | cut -d. -f1); \
	minor=$$(echo $$current_version | cut -d. -f2); \
	patch=$$(echo $$current_version | cut -d. -f3); \
	new_patch=$$((patch + 1)); \
	new_version="$${major}.$${minor}.$${new_patch}"; \
	echo $$new_version > VERSION; \
	echo "✓ Version: $$current_version -> $$new_version"

# Bump minor version (0.1.0 -> 0.2.0)
bump-minor:
	@echo "Bumping minor version..."
	@current_version=$$(cat VERSION); \
	major=$$(echo $$current_version | cut -d. -f1); \
	minor=$$(echo $$current_version | cut -d. -f2); \
	new_minor=$$((minor + 1)); \
	new_version="$${major}.$${new_minor}.0"; \
	echo $$new_version > VERSION; \
	echo "✓ Version: $$current_version -> $$new_version"

# Bump major version (0.1.0 -> 1.0.0)
bump-major:
	@echo "Bumping major version..."
	@current_version=$$(cat VERSION); \
	major=$$(echo $$current_version | cut -d. -f1); \
	new_major=$$((major + 1)); \
	new_version="$${new_major}.0.0"; \
	echo $$new_version > VERSION; \
	echo "✓ Version: $$current_version -> $$new_version"

# Shorthand: make patch = bump and build sequentially
patch:
	@$(MAKE) bump-patch
	@$(MAKE) build

# Shorthand: make minor = bump and build sequentially
minor:
	@$(MAKE) bump-minor
	@$(MAKE) build

# Shorthand: make major = bump and build sequentially
major:
	@$(MAKE) bump-major
	@$(MAKE) build

# Install to ~/.local/bin for development
dev-install: build
	@mkdir -p $(HOME)/.local/bin
	@cp $(BIN_DIR)/cilo $(HOME)/.local/bin/cilo
	@echo "✓ Installed to ~/.local/bin/cilo"

# Install to /usr/local/bin (requires sudo)
install: build
	@echo "Installing to /usr/local/bin/cilo..."
	@cp $(BIN_DIR)/cilo /usr/local/bin/cilo
	@echo "✓ Installed to /usr/local/bin/cilo"

# Quick build for local development
dev:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/cilo main.go
	@echo "✓ Built $(BIN_DIR)/cilo"

# Clean build artifacts
clean:
	@rm -f $(BIN_DIR)/cilo
	@rm -f cilo
	@echo "✓ Cleaned build artifacts"

# Show current version
version:
	@echo "Current version: $(VERSION)"
	@echo "Commit:          $(GIT_COMMIT)"
	@echo "Date:            $(BUILD_DATE)"

# Run tests
test:
	go test ./...
