# Go Tunn Tunnel Makefile

# Variables
BINARY_NAME = tunn
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -ldflags="-s -w -X main.Version=$(VERSION)"
DIST_DIR = dist

# Platform configurations
PLATFORMS = windows/amd64 windows/arm64 linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

# Default target
.PHONY: all
all: build

# Build for current platform
.PHONY: build
build:
	go build $(LDFLAGS) -o $(BINARY_NAME).exe .

# Build for all platforms
.PHONY: build-all
build-all: clean-dist
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		output_dir=$(DIST_DIR)/$$os-$$arch; \
		mkdir -p $$output_dir; \
		if [ "$$os" = "windows" ]; then \
			ext=".exe"; \
		else \
			ext=""; \
		fi; \
		echo "Building $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch go build $(LDFLAGS) -o $$output_dir/$(BINARY_NAME)$$ext .; \
	done

# Create release packages
.PHONY: package
package: build-all
	@mkdir -p $(DIST_DIR)/packages
	@for platform in $(PLATFORMS); do \
		os=$$(echo $$platform | cut -d'/' -f1); \
		arch=$$(echo $$platform | cut -d'/' -f2); \
		dir=$(DIST_DIR)/$$os-$$arch; \
		if [ "$$os" = "windows" ]; then \
			cd $$dir && zip ../packages/$(BINARY_NAME)-$$os-$$arch.zip $(BINARY_NAME).exe; \
		else \
			cd $$dir && tar -czf ../packages/$(BINARY_NAME)-$$os-$$arch.tar.gz $(BINARY_NAME); \
		fi; \
	done

# Generate checksums and create release
.PHONY: release
release: package
	@cd $(DIST_DIR)/packages && sha256sum * > checksums.txt
	@echo "Release packages created in $(DIST_DIR)/packages/"
	@echo "Checksums:"
	@cat $(DIST_DIR)/packages/checksums.txt

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_NAME).exe $(BINARY_NAME)

.PHONY: clean-dist
clean-dist:
	rm -rf $(DIST_DIR)/

.PHONY: clean-all
clean-all: clean clean-dist

# Development targets
.PHONY: deps
deps:
	go mod download && go mod tidy

.PHONY: run
run: build
	./$(BINARY_NAME).exe

.PHONY: test
test:
	go test -v ./...

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: lint
lint:
	golangci-lint run

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build      - Build for current platform"
	@echo "  build-all  - Build for all platforms"
	@echo "  package    - Create release packages"
	@echo "  release    - Build, package and generate checksums"
	@echo "  clean      - Remove binary files"
	@echo "  clean-all  - Remove all build artifacts"
	@echo "  deps       - Download dependencies"
	@echo "  run        - Build and run application"
	@echo "  test       - Run tests"
	@echo "  fmt        - Format code"
	@echo "  lint       - Run linter"
