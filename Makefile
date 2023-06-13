# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOBIN=./bin
GORUN=$(GOCMD) run

# Name of your binary executable
BINARY_NAME=graphqls-to-asciidoc

all: test build

build:
	$(GOBUILD) -o $(GOBIN)/$(BINARY_NAME) -v

test:
	$(GOTEST) -v ./...

clean:
	rm -rf $(GOBIN)

# Run the code to build to test doc
test_doc:
	$(GORUN) main.go test/schema.graphql > test/schema.adoc

.PHONY: all build test clean
