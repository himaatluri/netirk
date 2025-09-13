# Netirk Makefile
# Cross-platform network monitoring tool

# Project information
PROJECT_NAME := netirk
BINARY_NAME := netirk
MAIN_PACKAGE := .
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || powershell -Command "Get-Date -Format 'yyyy-MM-ddTHH:mm:ssZ'")
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOCLEAN := $(GOCMD) clean
GOTEST := $(GOCMD) test
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt
GOVET := $(GOCMD) vet

# Build flags
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

# Directories
BUILD_DIR := build
DIST_DIR := dist
COVERAGE_DIR := coverage

# Test parameters
TEST_TIMEOUT := 30s
COVERAGE_FILE := $(COVERAGE_DIR)/coverage.out
COVERAGE_HTML := $(COVERAGE_DIR)/coverage.html

# Default target
.DEFAULT_GOAL := help

# Help target
.PHONY: help
help: ## Display this help message
	@echo "Netirk - Network Monitoring Tool"
	@echo "================================="
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-20s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Development targets
.PHONY: deps
deps: ## Download and install dependencies
	@echo "Installing dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "Dependencies installed successfully"

.PHONY: fmt
fmt: ## Format Go source code
	@echo "Formatting Go code..."
	$(GOFMT) ./...
	@echo "Code formatting completed"

.PHONY: vet
vet: ## Run go vet on the source code
	@echo "Running go vet..."
	$(GOVET) ./...
	@echo "Vet check completed"

.PHONY: lint
lint: fmt vet ## Run formatting and vetting

# Build targets
.PHONY: build
build: deps lint ## Build the binary for current platform
	@echo "Building $(BINARY_NAME) for current platform..."
	@if not exist "$(BUILD_DIR)" mkdir "$(BUILD_DIR)"
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME).exe $(MAIN_PACKAGE)
	@echo "Build completed: $(BUILD_DIR)/$(BINARY_NAME).exe"

.PHONY: build-release
build-release: deps lint test ## Build optimized release binary
	@echo "Building release version of $(BINARY_NAME)..."
	@if not exist "$(BUILD_DIR)" mkdir "$(BUILD_DIR)"
	$(GOBUILD) $(LDFLAGS) -ldflags "-s -w" -o $(BUILD_DIR)/$(BINARY_NAME).exe $(MAIN_PACKAGE)
	@echo "Release build completed: $(BUILD_DIR)/$(BINARY_NAME).exe"

.PHONY: install
install: build ## Install the binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(LDFLAGS) $(MAIN_PACKAGE)
	@echo "Installation completed"

# Cross-compilation targets
.PHONY: build-all
build-all: deps lint ## Build for all supported platforms
	@echo "Building for all platforms..."
	@if not exist "$(DIST_DIR)" mkdir "$(DIST_DIR)"
	
	@echo "Building for Windows (amd64)..."
	@set GOOS=windows&& set GOARCH=amd64&& $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PACKAGE)
	
	@echo "Building for Windows (386)..."
	@set GOOS=windows&& set GOARCH=386&& $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-windows-386.exe $(MAIN_PACKAGE)
	
	@echo "Building for Linux (amd64)..."
	@set GOOS=linux&& set GOARCH=amd64&& $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)
	
	@echo "Building for Linux (arm64)..."
	@set GOOS=linux&& set GOARCH=arm64&& $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 $(MAIN_PACKAGE)
	
	@echo "Building for macOS (amd64)..."
	@set GOOS=darwin&& set GOARCH=amd64&& $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	
	@echo "Building for macOS (arm64)..."
	@set GOOS=darwin&& set GOARCH=arm64&& $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)
	
	@echo "Cross-compilation completed. Binaries available in $(DIST_DIR)/"

# Test targets
.PHONY: test
test: ## Run all tests
	@echo "Running tests..."
	@if not exist "$(COVERAGE_DIR)" mkdir "$(COVERAGE_DIR)"
	$(GOTEST) -timeout $(TEST_TIMEOUT) -v ./...
	@echo "Tests completed"

.PHONY: test-coverage
test-coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@if not exist "$(COVERAGE_DIR)" mkdir "$(COVERAGE_DIR)"
	$(GOTEST) -timeout $(TEST_TIMEOUT) -coverprofile=$(COVERAGE_FILE) -covermode=atomic ./...
	$(GOCMD) tool cover -html=$(COVERAGE_FILE) -o $(COVERAGE_HTML)
	@echo "Coverage report generated: $(COVERAGE_HTML)"
	@echo "Opening coverage report..."
	@start $(COVERAGE_HTML)

.PHONY: test-race
test-race: ## Run tests with race detection
	@echo "Running tests with race detection..."
	$(GOTEST) -timeout $(TEST_TIMEOUT) -race -v ./...
	@echo "Race detection tests completed"

.PHONY: test-integration
test-integration: build ## Run integration tests
	@echo "Running integration tests..."
	$(GOTEST) -timeout 60s -tags=integration -v ./...
	@echo "Integration tests completed"

.PHONY: benchmark
benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...
	@echo "Benchmarks completed"

# Quality assurance targets
.PHONY: check
check: deps lint test ## Run all quality checks
	@echo "All quality checks passed!"

.PHONY: security
security: ## Run security checks (requires gosec)
	@echo "Running security checks..."
	@where gosec >nul 2>&1 || (echo "gosec not found. Install with: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest" && exit 1)
	gosec ./...
	@echo "Security checks completed"

# Utility targets
.PHONY: clean
clean: ## Clean build artifacts and temporary files
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	@if exist "$(BUILD_DIR)" rmdir /s /q "$(BUILD_DIR)"
	@if exist "$(DIST_DIR)" rmdir /s /q "$(DIST_DIR)"
	@if exist "$(COVERAGE_DIR)" rmdir /s /q "$(COVERAGE_DIR)"
	@if exist "*.exe" del /q "*.exe"
	@if exist "*.test" del /q "*.test"
	@if exist "*.out" del /q "*.out"
	@echo "Clean completed"

.PHONY: version
version: ## Display version information
	@echo "Project: $(PROJECT_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Git Commit: $(GIT_COMMIT)"
	@echo "Go Version: $(shell $(GOCMD) version)"

.PHONY: info
info: ## Display project information
	@echo "Project Information"
	@echo "==================="
	@echo "Name: $(PROJECT_NAME)"
	@echo "Binary: $(BINARY_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Main Package: $(MAIN_PACKAGE)"
	@echo "Build Directory: $(BUILD_DIR)"
	@echo "Distribution Directory: $(DIST_DIR)"
	@echo ""
	@echo "Go Environment"
	@echo "=============="
	@$(GOCMD) env GOOS GOARCH GOVERSION GOROOT GOPATH

# Development workflow targets
.PHONY: dev
dev: clean deps build test ## Complete development workflow
	@echo "Development workflow completed successfully!"

.PHONY: release
release: clean deps build-release test-coverage ## Prepare release build
	@echo "Release preparation completed!"
	@echo "Binary available at: $(BUILD_DIR)/$(BINARY_NAME).exe"

.PHONY: ci
ci: deps lint test-race test-coverage ## Continuous integration workflow
	@echo "CI workflow completed successfully!"

# Example and documentation targets
.PHONY: examples
examples: build ## Generate example configuration files
	@echo "Generating example configurations..."
	@if not exist "examples" mkdir "examples"
	$(BUILD_DIR)/$(BINARY_NAME).exe validate --generate-example examples/generated-example.yaml
	@echo "Example configurations generated in examples/ directory"

.PHONY: demo
demo: build examples ## Run a quick demo of the tool
	@echo "Running netirk demo..."
	@echo "Checking connectivity to google.com..."
	$(BUILD_DIR)/$(BINARY_NAME).exe check https://google.com
	@echo ""
	@echo "Tracing network path to github.com..."
	$(BUILD_DIR)/$(BINARY_NAME).exe trace https://github.com
	@echo ""
	@echo "Demo completed! Check examples/ for configuration files."

# Docker targets (if needed in the future)
.PHONY: docker-build
docker-build: ## Build Docker image (requires Dockerfile)
	@echo "Building Docker image..."
	@if not exist "Dockerfile" (echo "Dockerfile not found" && exit 1)
	docker build -t $(PROJECT_NAME):$(VERSION) .
	docker build -t $(PROJECT_NAME):latest .
	@echo "Docker image built: $(PROJECT_NAME):$(VERSION)"

# Maintenance targets
.PHONY: update-deps
update-deps: ## Update all dependencies
	@echo "Updating dependencies..."
	$(GOGET) -u ./...
	$(GOMOD) tidy
	@echo "Dependencies updated"

.PHONY: verify-deps
verify-deps: ## Verify dependencies
	@echo "Verifying dependencies..."
	$(GOMOD) verify
	@echo "Dependencies verified"

# Quick targets for common workflows
.PHONY: quick-build
quick-build: fmt build ## Quick build without full checks

.PHONY: quick-test
quick-test: fmt test ## Quick test without full checks

.PHONY: full-check
full-check: clean deps lint test-race test-coverage security ## Full quality check suite