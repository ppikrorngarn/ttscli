.PHONY: all build clean run test test-race lint check help

# The name of the binary
BINARY_NAME=ttscli
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

all: build

## build: Build the CLI binary
build:
	@echo "Building $(BINARY_NAME)..."
	@go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) ./cmd/ttscli

## clean: Remove the compiled binary
clean:
	@echo "Cleaning up..."
	@rm -f $(BINARY_NAME)
	@rm -f *.mp3

## run: Run the CLI (e.g., make run ARGS="--text 'Hello' --play")
run: build
	@./$(BINARY_NAME) $(ARGS)

## test: Run tests
test:
	@go test -v ./...

## test-race: Run tests with the race detector
test-race:
	@go test -race ./...

## lint: Run static analysis checks
lint:
	@staticcheck ./...

## check: Run vet, tests, race tests, and lint
check:
	@go vet ./...
	@go test ./...
	@go test -race ./...
	@staticcheck ./...

## help: Show this help message
help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'
