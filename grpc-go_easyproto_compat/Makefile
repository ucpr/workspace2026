.PHONY: generate test bench tools

GOBIN := $(CURDIR)/bin

tools:
	GOBIN=$(GOBIN) go install github.com/bufbuild/buf/cmd/buf@latest
	GOBIN=$(GOBIN) go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

generate: tools
	PATH=$(GOBIN):$$PATH $(GOBIN)/buf generate

test:
	go test -v ./...

bench:
	go test -bench=. -benchmem -count=5 ./...
