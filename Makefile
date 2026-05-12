# m — project Makefile
# Run `make` for the default build; `make help` for all targets.

BINARY   := m
CMD      := ./cmd/m
VERSION  ?= dev
LDFLAGS  := -s -w -X main.Version=$(VERSION)

.PHONY: build test lint vet cover clean docker help desktop-dev desktop-build

build: ## Build the CLI binary (headless, no GUI)
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

build-gui: ## Build the binary with GUI support (for desktop installers)
	go build -tags gui -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) $(CMD)

desktop-dev: ## Run desktop app in development mode (requires Wails)
	@command -v wails >/dev/null 2>&1 || { echo "wails not found — install with: go install github.com/wailsapp/wails/v2/cmd/wails@latest"; exit 1; }
	wails dev

desktop-build: ## Build desktop app for current platform (requires Wails)
	@command -v wails >/dev/null 2>&1 || { echo "wails not found — install with: go install github.com/wailsapp/wails/v2/cmd/wails@latest"; exit 1; }
	wails build -clean
	@if [ "$$(uname)" = "Darwin" ]; then \
		codesign --force --deep --sign - --entitlements build/darwin/entitlements.plist build/bin/AgentCTL.app; \
		echo "Signed with entitlements"; \
	fi

desktop-build-all: ## Build desktop app for all platforms (macOS, Linux, Windows)
	@command -v wails >/dev/null 2>&1 || { echo "wails not found — install with: go install github.com/wailsapp/wails/v2/cmd/wails@latest"; exit 1; }
	wails build -platform darwin/universal -clean
	wails build -platform linux/amd64
	wails build -platform windows/amd64

frontend-deps: ## Install frontend dependencies
	cd frontend && npm install

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

docker: ## Build Docker image (headless CLI only)
	docker build --build-arg VERSION=$(VERSION) -t $(BINARY) .

validate: build ## Validate example agent docs
	./$(BINARY) validate examples/

clean: ## Remove build artifacts
	rm -f $(BINARY) coverage.txt
	rm -rf build/bin/
	rm -rf frontend/dist/ frontend/node_modules/

help: ## Show this help
	@grep -E '^[a-z][a-z_-]+:.*## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
