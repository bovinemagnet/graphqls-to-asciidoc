# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOBIN=./bin
GORUN=$(GOCMD) run
GOFMT=gofmt
GOLINT=golangci-lint

# Name of your binary executable
BINARY_NAME=graphqls-to-asciidoc

# Version info
VERSION ?= $(shell git describe --tags --always --dirty)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS = -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

all: test build

build:
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o $(GOBIN)/$(BINARY_NAME) -v

build-all:
	mkdir -p dist
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-linux-amd64
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-darwin-amd64
	GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o dist/$(BINARY_NAME)-windows-amd64.exe

test:
	$(GOTEST) -v ./...

test-coverage:
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

test-bench:
	$(GOTEST) -bench=. -benchmem ./...

lint:
	@if command -v $(GOLINT) >/dev/null 2>&1; then \
		$(GOLINT) run --timeout=5m; \
	else \
		echo "Warning: golangci-lint not found. Run 'make install-tools' to install it."; \
		echo "Skipping lint check..."; \
	fi

fmt:
	$(GOFMT) -s -w .

fmt-check:
	@if [ -n "$$($(GOFMT) -l .)" ]; then \
		echo "Go code is not formatted:"; \
		$(GOFMT) -d .; \
		exit 1; \
	fi

vet:
	$(GOCMD) vet ./...

mod-tidy:
	$(GOCMD) mod tidy

security:
	@if command -v gosec >/dev/null 2>&1; then \
		gosec ./...; \
	else \
		echo "Warning: gosec not found. Run 'make install-tools' to install it."; \
		echo "Skipping security check..."; \
	fi

clean:
	rm -rf $(GOBIN) dist/ coverage.out coverage.html

# Run the code to build to test doc
test_doc:
	$(GORUN) main.go -schema test/schema.graphql > test/schema.adoc

# Validate that test doc generation works
validate-test-doc: build
	$(GOBIN)/$(BINARY_NAME) -schema test/schema.graphql > /tmp/test-output.adoc
	@if [ ! -s /tmp/test-output.adoc ]; then \
		echo "ERROR: Generated documentation is empty"; \
		exit 1; \
	fi
	@echo "✓ Test documentation generated successfully"

# Run all checks (CI-like)
check: fmt-check vet test test-coverage validate-test-doc lint security

# Install development tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest

# Docker build
docker-build:
	docker build -t $(BINARY_NAME):$(VERSION) .

.PHONY: all build build-all test test-coverage test-bench lint fmt fmt-check vet mod-tidy security clean test_doc validate-test-doc check install-tools docker-build
