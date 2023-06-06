help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

.PHONY: build
build:        ## Build application
	mkdir -p bin
	CGO_ENABLED=0  go build -o ./bin/downloader main.go
.PHONY: lint
lint:         ## Run lint checks
	golangci-lint run -c ./.golangci.yml -v 
	
.PHONY: test
test:         ## Run unit tests
	go test -timeout 30s -race ./...
