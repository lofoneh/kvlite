# Makefile for kvlite

.PHONY: all build test bench clean run install help client

# Variables
BINARY_NAME=kvlite
BINARY_DIR=bin
CLIENT_NAME=test_client
VERSION=0.1.0

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GORUN=$(GOCMD) run

all: test build

## build: Build the kvlite binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BINARY_DIR)
	@$(GOBUILD) -o $(BINARY_DIR)/$(BINARY_NAME) -v ./cmd/kvlite/
	@echo "✓ Build complete: $(BINARY_DIR)/$(BINARY_NAME)"

## build-client: Build the test client
build-client:
	@echo "Building test client..."
	@mkdir -p $(BINARY_DIR)
	@$(GOBUILD) -o $(BINARY_DIR)/$(CLIENT_NAME) -v ./scripts/test_client.go
	@echo "✓ Client built: $(BINARY_DIR)/$(CLIENT_NAME)"

## run: Build and run the server
run: build
	@echo "Starting kvlite server..."
	@./$(BINARY_DIR)/$(BINARY_NAME)

## client: Build and run the test client
client: build-client
	@./$(BINARY_DIR)/$(CLIENT_NAME)

## test: Run all tests
test:
	@echo "Running tests..."
	@$(GOTEST) -v ./...

## test-race: Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	@$(GOTEST) -race -v ./...

## test-cover: Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	@$(GOTEST) -cover ./...
	@$(GOTEST) -coverprofile=coverage.out ./...
	@$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

## bench: Run benchmarks
bench:
	@echo "Running Go benchmarks..."
	@$(GOTEST) -bench=. -benchmem ./internal/store/

## bench-all: Run all benchmarks including network tests
bench-all: bench
	@echo ""
	@echo "Running network benchmarks..."
	@chmod +x scripts/bench.sh 2>/dev/null || true
	@bash scripts/bench.sh || echo "Network benchmarks skipped (check if server is running)"

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BINARY_DIR)
	@rm -f coverage.out coverage.html
	@$(GOCMD) clean
	@echo "✓ Clean complete"

## install: Install kvlite to GOPATH/bin
install:
	@echo "Installing $(BINARY_NAME)..."
	@$(GOCMD) install ./cmd/kvlite/
	@echo "✓ Installed to $(GOPATH)/bin/$(BINARY_NAME)"

## fmt: Format code
fmt:
	@echo "Formatting code..."
	@$(GOCMD) fmt ./...
	@echo "✓ Format complete"

## vet: Run go vet
vet:
	@echo "Running go vet..."
	@$(GOCMD) vet ./...
	@echo "✓ Vet complete"

## lint: Run golangci-lint (requires golangci-lint to be installed)
lint:
	@echo "Running linter..."
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest" && exit 1)
	@golangci-lint run ./...
	@echo "✓ Lint complete"

## mod-tidy: Tidy go modules
mod-tidy:
	@echo "Tidying modules..."
	@$(GOMOD) tidy
	@echo "✓ Modules tidied"

## deps: Download dependencies
deps:
	@echo "Downloading dependencies..."
	@$(GOMOD) download
	@echo "✓ Dependencies downloaded"

## check: Run all checks (fmt, vet, test)
check: fmt vet test
	@echo "✓ All checks passed"

## help: Show this help message
help:
	@echo "kvlite Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

.DEFAULT_GOAL := help