# Project variables
BINARY_NAME=api
BUILD_DIR=bin
MAIN_PATH=cmd/api/main.go

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# Environment variables for build
CGO_ENABLED=0

.PHONY: all build run test clean deps lint help

all: build

help: ## Display this help screen
	@grep -h -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build the binary
	@echo "Building..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)

run: build ## Build and run the binary
	@echo "Running..."
	./$(BUILD_DIR)/$(BINARY_NAME)

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out

clean: ## Remove build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

deps: ## Download and tidy dependencies
	@echo "Tidying dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

lint: ## Run golangci-lint
	@if command -v golangci-lint > /dev/null; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found. Install it from https://golangci-lint.run/usage/install/"; \
		exit 1; \
	fi

# Docker targets (Optional)
docker-build: ## Build docker image
	docker build -t $(BINARY_NAME):latest .

docker-run: ## Run docker container
	docker run -p 8080:8080 $(BINARY_NAME):latest
