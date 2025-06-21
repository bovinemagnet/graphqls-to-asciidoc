# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOBIN=./bin
GORUN=$(GOCMD) run
GOFMT=gofmt
GOLINT=golangci-lint

# Color definitions
GREEN=\033[0;32m
RED=\033[0;31m
YELLOW=\033[0;33m
BLUE=\033[0;34m
NC=\033[0m # No Color

# Name of your binary executable
BINARY_NAME=graphqls-to-asciidoc

# Version info
VERSION ?= $(shell git describe --tags --always --dirty)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS = -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

all: test build

build:
	@echo "$(BLUE)Building $(BINARY_NAME)...$(NC)"
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(GOBIN)/$(BINARY_NAME) -v
	@echo "$(GREEN)✓ Build completed successfully$(NC)"

build-all:
	@echo "$(BLUE)Building for multiple platforms...$(NC)"
	mkdir -p dist
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-amd64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-darwin-amd64
	GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-windows-amd64.exe
	@echo "$(GREEN)✓ Multi-platform build completed successfully$(NC)"

test:
	@echo "$(BLUE)Running tests...$(NC)"
	@$(GOTEST) -v ./... 2>&1 | tee /tmp/test-output.log; \
	if [ $${PIPESTATUS[0]} -eq 0 ]; then \
		echo "$(GREEN)✓ All tests passed!$(NC)"; \
	else \
		echo "$(RED)✗ Some tests failed!$(NC)"; \
		echo "$(YELLOW)Check the output above for details.$(NC)"; \
		exit 1; \
	fi

test-coverage:
	@echo "$(BLUE)Running tests with coverage...$(NC)"
	@$(GOTEST) -v -race -coverprofile=coverage.out ./... 2>&1 | tee /tmp/test-coverage-output.log; \
	if [ $${PIPESTATUS[0]} -eq 0 ]; then \
		echo "$(GREEN)✓ All tests passed with coverage!$(NC)"; \
		$(GOCMD) tool cover -html=coverage.out -o coverage.html; \
		echo "$(BLUE)Coverage report generated: coverage.html$(NC)"; \
	else \
		echo "$(RED)✗ Some tests failed!$(NC)"; \
		echo "$(YELLOW)Check the output above for details.$(NC)"; \
		exit 1; \
	fi

test-bench:
	@echo "$(BLUE)Running benchmark tests...$(NC)"
	$(GOTEST) -bench=. -benchmem ./...
	@echo "$(GREEN)✓ Benchmark tests completed$(NC)"

lint:
	@echo "$(BLUE)Running linter...$(NC)"
	@if command -v $(GOLINT) >/dev/null 2>&1; then \
		if $(GOLINT) run --timeout=5m; then \
			echo "$(GREEN)✓ Linting passed!$(NC)"; \
		else \
			echo "$(RED)✗ Linting failed!$(NC)"; \
			exit 1; \
		fi; \
	else \
		echo "$(YELLOW)Warning: golangci-lint not found. Run 'make install-tools' to install it.$(NC)"; \
		echo "$(YELLOW)Skipping lint check...$(NC)"; \
	fi

fmt:
	@echo "$(BLUE)Formatting code...$(NC)"
	$(GOFMT) -s -w .
	@echo "$(GREEN)✓ Code formatting completed$(NC)"

fmt-check:
	@echo "$(BLUE)Checking code formatting...$(NC)"
	@if [ -n "$$($(GOFMT) -l .)" ]; then \
		echo "$(RED)✗ Go code is not formatted:$(NC)"; \
		$(GOFMT) -d .; \
		exit 1; \
	else \
		echo "$(GREEN)✓ Code formatting is correct$(NC)"; \
	fi

vet:
	@echo "$(BLUE)Running go vet...$(NC)"
	@if $(GOCMD) vet ./...; then \
		echo "$(GREEN)✓ Go vet passed!$(NC)"; \
	else \
		echo "$(RED)✗ Go vet failed!$(NC)"; \
		exit 1; \
	fi

mod-tidy:
	@echo "$(BLUE)Tidying modules...$(NC)"
	$(GOCMD) mod tidy
	@echo "$(GREEN)✓ Modules tidied$(NC)"

security:
	@echo "$(BLUE)Running security checks...$(NC)"
	@if command -v gosec >/dev/null 2>&1; then \
		if gosec ./...; then \
			echo "$(GREEN)✓ Security checks passed!$(NC)"; \
		else \
			echo "$(RED)✗ Security checks failed!$(NC)"; \
			exit 1; \
		fi; \
	else \
		echo "$(YELLOW)Warning: gosec not found. Run 'make install-tools' to install it.$(NC)"; \
		echo "$(YELLOW)Skipping security check...$(NC)"; \
	fi

clean:
	@echo "$(BLUE)Cleaning build artifacts...$(NC)"
	rm -rf $(GOBIN) dist/ coverage.out coverage.html
	@echo "$(GREEN)✓ Clean completed$(NC)"

# Run the code to build to test doc
test_doc:
	@echo "$(BLUE)Generating test documentation...$(NC)"
	$(GORUN) main.go -schema test/schema.graphql > test/schema.adoc
	@echo "$(GREEN)✓ Test documentation generated$(NC)"

# Validate that test doc generation works
validate-test-doc: build
	@echo "$(BLUE)Validating test documentation generation...$(NC)"
	$(GOBIN)/$(BINARY_NAME) -schema test/schema.graphql > /tmp/test-output.adoc
	@if [ ! -s /tmp/test-output.adoc ]; then \
		echo "$(RED)✗ ERROR: Generated documentation is empty$(NC)"; \
		exit 1; \
	fi
	@echo "$(GREEN)✓ Test documentation generated successfully$(NC)"

# Run all checks (CI-like)
check: fmt-check vet test test-coverage validate-test-doc lint security
	@echo "$(GREEN)✓ All checks completed successfully!$(NC)"

# Install development tools
install-tools:
	@echo "$(BLUE)Installing development tools...$(NC)"
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	@echo "$(GREEN)✓ Development tools installed$(NC)"

# Docker build
docker-build:
	@echo "$(BLUE)Building Docker image...$(NC)"
	docker build -t $(BINARY_NAME):$(VERSION) .
	@echo "$(GREEN)✓ Docker image built successfully$(NC)"

.PHONY: all build build-all test test-coverage test-bench lint fmt fmt-check vet mod-tidy security clean test_doc validate-test-doc check install-tools docker-build
