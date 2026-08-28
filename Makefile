GOPATH_BIN := $(shell go env GOPATH)/bin
GOLANGCI_LINT := $(GOPATH_BIN)/golangci-lint
GOIMPORTS := $(GOPATH_BIN)/goimports


.PHONY: fmt lint vet test test-integration build tidy check

fmt:
	$(GOIMPORTS) -w -local github.com/Hinkal-Protocol/hinkal-go .
	gofmt -w .

lint:
	$(GOLANGCI_LINT) run ./...

vet:
	go vet ./...

test:
	go test -v -race -short ./...

test-integration:
	go test -v -race -timeout 120s -run TestIntegration ./test/integration/...

build:
	go build ./...

tidy:
	go mod tidy

check: fmt vet lint test
