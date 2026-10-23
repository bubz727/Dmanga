.PHONY: build test clean run

BINARY=Dmanga.exe
SRC=./cmd/dmanga

VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || git describe --tags 2>/dev/null || echo dev)
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_TIME ?= $(shell git log -1 --format=%cI 2>/dev/null || echo unknown)

LDFLAGS=-s -w -X 'dmanga/internal/cli.Version=$(VERSION)' -X 'dmanga/internal/cli.GitCommit=$(GIT_COMMIT)' -X 'dmanga/internal/cli.BuildTime=$(BUILD_TIME)'

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) $(SRC)

run:
	go run -ldflags="$(LDFLAGS)" $(SRC)

test:
	go test -v ./...

clean:
	if exist $(BINARY) del $(BINARY)
