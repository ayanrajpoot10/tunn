# Go Tunn Tunnel Makefile

# Variables
BINARY_NAME=tunn
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags="-s -w -X main.Version=$(VERSION)"

# Default target
.PHONY: all
all: build

# Build for current platform
.PHONY: build
build:
	go build $(LDFLAGS) -o $(BINARY_NAME).exe .

# Build for all platforms
.PHONY: build-all
build-all: build-windows build-linux build-darwin build-freebsd

.PHONY: build-windows
build-windows:
	mkdir -p dist/windows-amd64 dist/windows-386 dist/windows-arm64
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/windows-amd64/$(BINARY_NAME).exe .
	GOOS=windows GOARCH=386 go build $(LDFLAGS) -o dist/windows-386/$(BINARY_NAME).exe .
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o dist/windows-arm64/$(BINARY_NAME).exe .

.PHONY: build-linux
build-linux:
	mkdir -p dist/linux-amd64 dist/linux-386 dist/linux-arm64 dist/linux-arm
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o dist/linux-amd64/$(BINARY_NAME) .
	GOOS=linux GOARCH=386 go build $(LDFLAGS) -o dist/linux-386/$(BINARY_NAME) .
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o dist/linux-arm64/$(BINARY_NAME) .
	GOOS=linux GOARCH=arm go build $(LDFLAGS) -o dist/linux-arm/$(BINARY_NAME) .

.PHONY: build-darwin
build-darwin:
	mkdir -p dist/darwin-amd64 dist/darwin-arm64
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o dist/darwin-amd64/$(BINARY_NAME) .
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o dist/darwin-arm64/$(BINARY_NAME) .

.PHONY: build-freebsd
build-freebsd:
	mkdir -p dist/freebsd-amd64
	GOOS=freebsd GOARCH=amd64 go build $(LDFLAGS) -o dist/freebsd-amd64/$(BINARY_NAME) .

# Create release archives
.PHONY: package
package: build-all
	mkdir -p dist/packages
	# Windows packages
	cd dist/windows-amd64 && zip ../packages/$(BINARY_NAME)-windows-amd64.zip $(BINARY_NAME).exe
	cd dist/windows-386 && zip ../packages/$(BINARY_NAME)-windows-386.zip $(BINARY_NAME).exe
	cd dist/windows-arm64 && zip ../packages/$(BINARY_NAME)-windows-arm64.zip $(BINARY_NAME).exe
	# Linux packages
	cd dist/linux-amd64 && tar -czf ../packages/$(BINARY_NAME)-linux-amd64.tar.gz $(BINARY_NAME)
	cd dist/linux-386 && tar -czf ../packages/$(BINARY_NAME)-linux-386.tar.gz $(BINARY_NAME)
	cd dist/linux-arm64 && tar -czf ../packages/$(BINARY_NAME)-linux-arm64.tar.gz $(BINARY_NAME)
	cd dist/linux-arm && tar -czf ../packages/$(BINARY_NAME)-linux-arm.tar.gz $(BINARY_NAME)
	# macOS packages
	cd dist/darwin-amd64 && tar -czf ../packages/$(BINARY_NAME)-darwin-amd64.tar.gz $(BINARY_NAME)
	cd dist/darwin-arm64 && tar -czf ../packages/$(BINARY_NAME)-darwin-arm64.tar.gz $(BINARY_NAME)
	# FreeBSD packages
	cd dist/freebsd-amd64 && tar -czf ../packages/$(BINARY_NAME)-freebsd-amd64.tar.gz $(BINARY_NAME)

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
	rm -f $(BINARY_NAME).exe $(BINARY_NAME)
	rm -rf dist/

# Install dependencies
.PHONY: deps
deps:
	go mod download
	go mod tidy

# Run the application
.PHONY: run
run: build
	./$(BINARY_NAME).exe

# Run with help
.PHONY: run-help
run-help: build
	./$(BINARY_NAME).exe --help

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
