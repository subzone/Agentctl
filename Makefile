# m — project Makefile
# Run `make` for the default build; `make help` for all targets.

BINARY   := m
CMD      := ./cmd/m
VERSION  ?= dev
LDFLAGS  := -s -w -X main.Version=$(VERSION)

.PHONY: build test lint vet cover clean docker help

build: ## Build the binary
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

test: ## Run all tests
	go test ./...

race: ## Run tests with race detector
	go test -race ./...

cover: ## Run tests with coverage report
	go test -coverprofile=coverage.txt ./...
	go tool cover -func=coverage.txt | tail -1

vet: ## Run go vet
	go vet ./...

lint: vet ## Run golangci-lint (install with: brew install golangci-lint)
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not found — install with: brew install golangci-lint"; exit 1; }
	golangci-lint run

docker: ## Build Docker image
	docker build --build-arg VERSION=$(VERSION) -t $(BINARY) .

validate: build ## Validate example agent docs
	./$(BINARY) validate examples/

clean: ## Remove build artifacts
	rm -f $(BINARY) coverage.txt

help: ## Show this help
	@grep -E '^[a-z][a-z_-]+:.*## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
