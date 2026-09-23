GO ?= go

.PHONY: all build test test-race vet fmt fmt-check check clean

all: check

build:
	$(GO) build ./cmd/catapult

test:
	$(GO) test ./...

test-race:
	$(GO) test -race ./...

vet:
	$(GO) vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

fmt-check:
	test -z "$$(gofmt -l .)"

check: fmt-check vet test

clean:
	$(GO) clean -cache
