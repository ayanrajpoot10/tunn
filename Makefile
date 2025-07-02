# Go Tunn Tunnel Makefile

# Variables
BINARY_NAME=tunn
BINARY_WINDOWS=$(BINARY_NAME).exe
BINARY_LINUX=$(BINARY_NAME)
BINARY_DARWIN=$(BINARY_NAME)_darwin
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags="-s -w -X main.Version=$(VERSION)"

# Default target
.PHONY: all
all: build

# Build for current platform
.PHONY: build
build:
	go build $(LDFLAGS) -o $(BINARY_WINDOWS) .

# Build for all platforms
.PHONY: build-all
build-all: build-windows build-linux build-darwin build-freebsd

.PHONY: build-windows
build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-amd64.exe .
	GOOS=windows GOARCH=386 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-386.exe .
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-windows-arm64.exe .

.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=386 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-386 .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm64 .
	GOOS=linux GOARCH=arm go build $(LDFLAGS) -o dist/$(BINARY_NAME)-linux-arm .

.PHONY: build-darwin
build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-darwin-arm64 .

.PHONY: build-freebsd
build-freebsd:
	GOOS=freebsd GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY_NAME)-freebsd-amd64 .

# Create release archives
.PHONY: package
package: build-all
	mkdir -p dist/packages
	# Windows packages
	cd dist && zip packages/$(BINARY_NAME)-windows-amd64.zip $(BINARY_NAME)-windows-amd64.exe
	cd dist && zip packages/$(BINARY_NAME)-windows-386.zip $(BINARY_NAME)-windows-386.exe
	cd dist && zip packages/$(BINARY_NAME)-windows-arm64.zip $(BINARY_NAME)-windows-arm64.exe
	# Linux packages
	cd dist && tar -czf packages/$(BINARY_NAME)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	cd dist && tar -czf packages/$(BINARY_NAME)-linux-386.tar.gz $(BINARY_NAME)-linux-386
	cd dist && tar -czf packages/$(BINARY_NAME)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64
	cd dist && tar -czf packages/$(BINARY_NAME)-linux-arm.tar.gz $(BINARY_NAME)-linux-arm
	# macOS packages
	cd dist && tar -czf packages/$(BINARY_NAME)-darwin-amd64.tar.gz $(BINARY_NAME)-darwin-amd64
	cd dist && tar -czf packages/$(BINARY_NAME)-darwin-arm64.tar.gz $(BINARY_NAME)-darwin-arm64
	# FreeBSD packages
	cd dist && tar -czf packages/$(BINARY_NAME)-freebsd-amd64.tar.gz $(BINARY_NAME)-freebsd-amd64

# Generate checksums
.PHONY: checksums
checksums: package
	cd dist/packages && sha256sum * > checksums.txt

# Create a local release (build + package + checksums)
.PHONY: release
release: clean checksums
	@echo "Release packages created in dist/packages/"
	@echo "Checksums:"
	@cat dist/packages/checksums.txt

# Release to GitHub (requires version tag)
.PHONY: github-release
github-release:
	@if [ -z "$(TAG)" ]; then \
		echo "Error: TAG is required. Usage: make github-release TAG=v1.0.0"; \
		exit 1; \
	fi
	@echo "Creating GitHub release $(TAG)..."
	git tag -a $(TAG) -m "Release $(TAG)"
	git push origin $(TAG)
	@echo "GitHub Actions will automatically create the release"

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_WINDOWS) $(BINARY_LINUX) $(BINARY_DARWIN)
	rm -rf dist/

# Install dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy

# Run the application
.PHONY: run
run: build
	./$(BINARY_WINDOWS)

# Run with help
.PHONY: run-help
run-help: build
	./$(BINARY_WINDOWS) --help

# Run tests
.PHONY: test
test:
	go test -v ./...

# Format code
.PHONY: fmt
fmt:
	go fmt ./...

# Lint code
.PHONY: lint
lint:
	golangci-lint run

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build           - Build for current platform"
	@echo "  build-all       - Build for all platforms (Windows, Linux, macOS, FreeBSD)"
	@echo "  build-windows   - Build for Windows (amd64, 386, arm64)"
	@echo "  build-linux     - Build for Linux (amd64, 386, arm64, arm)"
	@echo "  build-darwin    - Build for macOS (amd64, arm64)"
	@echo "  build-freebsd   - Build for FreeBSD (amd64)"
	@echo "  package         - Create release archives for all platforms"
	@echo "  checksums       - Generate SHA256 checksums for packages"
	@echo "  release         - Create a complete local release (build + package + checksums)"
	@echo "  github-release  - Create and push a Git tag for GitHub release (use TAG=v1.0.0)"
	@echo "  clean           - Remove build artifacts"
	@echo "  deps            - Download and organize dependencies"
	@echo "  run             - Build and run the application"
	@echo "  run-help        - Build and show application help"
	@echo "  test            - Run tests"
	@echo "  fmt             - Format Go code"
	@echo "  lint            - Run linter"
	@echo "  help            - Show this help message"
