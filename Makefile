# Go Tunn Tunnel Makefile

# Variables
BINARY_NAME=tunn
BINARY_WINDOWS=$(BINARY_NAME).exe
BINARY_LINUX=$(BINARY_NAME)
BINARY_DARWIN=$(BINARY_NAME)_darwin

# Default target
.PHONY: all
all: build

# Build for current platform
.PHONY: build
build:
	go build -o $(BINARY_WINDOWS) .

# Build for all platforms
.PHONY: build-all
build-all: build-windows build-linux build-darwin

.PHONY: build-windows
build-windows:
	GOOS=windows GOARCH=amd64 go build -o $(BINARY_WINDOWS) .

.PHONY: build-linux
build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_LINUX) .

.PHONY: build-darwin
build-darwin:
	GOOS=darwin GOARCH=amd64 go build -o $(BINARY_DARWIN) .

# Clean build artifacts
.PHONY: clean
clean:
	rm -f $(BINARY_WINDOWS) $(BINARY_LINUX) $(BINARY_DARWIN)

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
	@echo "  build-all       - Build for Windows, Linux, and macOS"
	@echo "  clean           - Remove build artifacts"
	@echo "  deps            - Download and organize dependencies"
	@echo "  run             - Build and run the application"
	@echo "  run-help        - Build and show application help"
	@echo "  test            - Run tests"
	@echo "  fmt             - Format Go code"
	@echo "  lint            - Run linter"
	@echo "  help            - Show this help message"
