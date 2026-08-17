.PHONY: build test lint fmt tidy check-release build-lambdas clean tf-init tf-plan tf-apply tf-fmt

GOOS ?= linux
GOARCH ?= arm64
BIN_DIR := bin
CMDS := api check-release fetch-context analyze-release persist-release

## Run unit tests.
test:
	go test ./... -race -cover

## Static analysis. Uses go vet always; runs golangci-lint too if installed.
lint:
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then golangci-lint run ./...; \
	else echo "golangci-lint not installed, skipping (go vet already ran)"; fi

fmt:
	gofmt -w .

tidy:
	go mod tidy

## Compile every binary for the host platform (fast local sanity build).
build:
	go build ./...

## Run the release checker locally, without any Lambda runtime, against
## a single repository - spec section 24's `make check-release`.
##
##   make check-release REPOSITORY=open-telemetry/opentelemetry-collector-contrib
check-release:
	go run ./cmd/check-release

## Cross-compile every Lambda handler for linux/arm64 (Graviton, cheaper)
## and zip each one, ready for `terraform apply` (infra/terraform reads
## these zips via the archive path each *_lambda_function resource
## points at).
build-lambdas: clean
	@mkdir -p $(BIN_DIR)
	@for cmd in $(CMDS); do \
		echo "building $$cmd"; \
		GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 go build -o $(BIN_DIR)/$$cmd/bootstrap ./cmd/$$cmd; \
		(cd $(BIN_DIR)/$$cmd && zip -q -X ../$$cmd.zip bootstrap); \
	done

clean:
	rm -rf $(BIN_DIR)

## Terraform convenience wrappers.
tf-init:
	cd infra/terraform && terraform init

tf-plan:
	cd infra/terraform && terraform plan

tf-apply:
	cd infra/terraform && terraform apply

tf-fmt:
	cd infra/terraform && terraform fmt -recursive
