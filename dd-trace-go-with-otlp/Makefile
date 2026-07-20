BIN := $(CURDIR)/bin
BUF := $(BIN)/buf

# Tool versions (buf >= 1.72 needs Go >= 1.25.10; pin to a Go 1.25.5-compatible one).
BUF_VERSION               := v1.55.1
PROTOC_GEN_GO_VERSION     := v1.36.6
PROTOC_GEN_GO_GRPC_VERSION := v1.5.1

.PHONY: tools
tools: ## Install buf + protoc plugins into ./bin
	GOBIN=$(BIN) go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	GOBIN=$(BIN) go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)
	GOBIN=$(BIN) go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)

.PHONY: generate
generate: ## Generate Go code from proto with buf
	PATH="$(BIN):$$PATH" $(BUF) lint
	PATH="$(BIN):$$PATH" $(BUF) generate

.PHONY: build
build: ## Build all binaries
	go build ./...

.PHONY: test
test: ## Run tests
	go test ./...

.PHONY: up
up: ## Start the whole stack with docker compose
	docker compose up --build

.PHONY: down
down: ## Stop the stack and remove volumes
	docker compose down -v

.PHONY: smoke
smoke: ## Hit the gateway to generate a trace
	curl -sS -XPOST localhost:8080/orders -d '{"item":"book","quantity":3}'; echo
	curl -sS localhost:8080/orders; echo
	curl -sS localhost:8080/orders/1; echo

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-12s\033[0m %s\n", $$1, $$2}'
