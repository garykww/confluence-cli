.PHONY: build build-all test lint fmt clean install

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags="-s -w -X main.buildVersion=$(VERSION)"
BINARY   := confluence-cli

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/confluence-cli

build-all:
	GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64   ./cmd/confluence-cli
	GOOS=linux   GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/$(BINARY)-linux-arm64   ./cmd/confluence-cli
	GOOS=darwin  GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64  ./cmd/confluence-cli
	GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64  ./cmd/confluence-cli
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe ./cmd/confluence-cli

test:
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...

lint:
	golangci-lint run

fmt:
	gofmt -s -w .

clean:
	rm -f $(BINARY) coverage.txt
	rm -rf dist/

install: build
	cp $(BINARY) $(shell go env GOPATH)/bin/$(BINARY)
