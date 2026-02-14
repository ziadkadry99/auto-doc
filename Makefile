BINARY_NAME=autodoc
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS=-ldflags "-X github.com/ziadkadry99/auto-doc/cmd.Version=$(VERSION)"

.PHONY: build test lint clean tidy install release dist

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) .

test:
	go test -race ./...

lint:
	golangci-lint run ./...

clean:
	rm -f $(BINARY_NAME)
	go clean

tidy:
	go mod tidy

install:
	go install $(LDFLAGS) .

release:
	goreleaser release --clean

dist:
	goreleaser build --snapshot --clean

.DEFAULT_GOAL := build
