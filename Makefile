.PHONY: all build clean run help

# The name of the binary
BINARY_NAME=ttscli

all: build

## build: Build the CLI binary
build:
	@echo "Building $(BINARY_NAME)..."
	@go build -o $(BINARY_NAME) ./cmd/ttscli

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

## help: Show this help message
help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' |  sed -e 's/^/ /'
