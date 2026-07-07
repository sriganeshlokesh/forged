VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: all fmt lint test build run wire mocks docker-build tidy

all: fmt lint test build

fmt:
	gofmt -s -w .

lint:
	@if ! command -v golangci-lint > /dev/null 2>&1; then \
		echo "golangci-lint not found. Install with: brew install golangci-lint"; \
		exit 1; \
	fi
	golangci-lint run ./...

test:
	go test -race -coverprofile=coverage.out ./...

build:
	CGO_ENABLED=0 go build -trimpath \
		-ldflags '-s -w -X github.com/sriganeshlokesh/forged/config.Version=$(VERSION)' \
		-o bin/forged ./cmd

run:
	go run ./cmd

wire:
	go tool wire ./adapter/dependency/

mocks:
	go tool mockery

docker-build:
	docker build --build-arg VERSION=$(VERSION) -t forged:local .

tidy:
	go mod tidy
