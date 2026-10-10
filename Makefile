BINARY  := dockprune
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOFLAGS := -trimpath

PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: all build build-all build-linux build-darwin build-windows \
	test test-race lint vet demo clean

all: lint test build

build:
	mkdir -p dist
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/$(BINARY) .

build-all:
	mkdir -p dist
	@$(foreach p,$(PLATFORMS), \
		echo ">> building $(p)"; \
		GOOS=$(word 1,$(subst /, ,$(p))) GOARCH=$(word 2,$(subst /, ,$(p))) \
		CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
			-o dist/$(BINARY)-$(subst /,-,$(p))$(if $(findstring windows,$(p)),.exe) .; \
	)

build-linux:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o dist/$(BINARY)-linux-amd64 .

build-darwin:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o dist/$(BINARY)-darwin-amd64 .

build-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o dist/$(BINARY)-windows-amd64.exe .

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, falling back to go vet"; \
		go vet ./...; \
	fi

# Regenerate assets/demo.gif. Starts the demo-only fake Docker API and points
# DOCKER_HOST at it, so no real daemon or resources are touched. Requires vhs.
demo: build
	@go build -o /tmp/dockprune-fakeapi ./demo/fakeapi
	@rm -f /tmp/dockprune-demo.sock
	@/tmp/dockprune-fakeapi --socket /tmp/dockprune-demo.sock >/dev/null 2>&1 & echo $$! > /tmp/dockprune-demo.pid; \
	for i in $$(seq 1 50); do [ -S /tmp/dockprune-demo.sock ] && break; sleep 0.1; done; \
	DOCKER_HOST=unix:///tmp/dockprune-demo.sock vhs demo.tape; \
	kill $$(cat /tmp/dockprune-demo.pid) 2>/dev/null || true

clean:
	rm -rf dist
