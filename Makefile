GO ?= go

.PHONY: all build test test-race vet fmt fmt-check check install-restart clean

all: check

build:
	$(GO) build ./cmd/catapult

install-restart:
	mkdir -p "$(HOME)/.local/bin"
	$(GO) build -o "$(HOME)/.local/bin/catapult" ./cmd/catapult
	"$(HOME)/.local/bin/catapult" service restart

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
