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

gomobile-bind:
	mkdir -p ../../dist/go/android
	GOFLAGS=-trimpath $(GOPATH_BIN)/gomobile bind -target=android/arm64,android/amd64 -androidapi 21 \
		-ldflags "-extldflags=-Wl,-z,max-page-size=16384" \
		-javapkg io.hinkal -o ../../dist/go/android/hinkal.aar $(GOMOBILE_PKGS)

gomobile-bind-ios:
	mkdir -p ../../dist/go/ios
	GOFLAGS=-trimpath $(GOPATH_BIN)/gomobile bind -target=ios,iossimulator \
		-o ../../dist/go/ios/Hinkal.xcframework $(GOMOBILE_PKGS)

check: fmt vet lint test
