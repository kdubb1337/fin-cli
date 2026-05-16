.PHONY: build fmt lint test test-integration ci tools install clean

CLI_NAME := fin
MODULE   := $(shell go list -m)

# Resolve dev tools. Prefer system install if present; fall back to .tools/
# (populated by `make tools`). Using explicit paths avoids relying on PATH
# propagation, which GNU Make 3.81 (default on macOS) does not handle reliably
# via `export PATH := ...` for recipe shells.
TOOLS         := $(CURDIR)/.tools
GOFUMPT       := $(shell command -v gofumpt 2>/dev/null || echo $(TOOLS)/gofumpt)
GOIMPORTS     := $(shell command -v goimports 2>/dev/null || echo $(TOOLS)/goimports)
GOLANGCI_LINT := $(shell command -v golangci-lint 2>/dev/null || echo $(TOOLS)/golangci-lint)

build:
	@mkdir -p bin
	go build -o bin/$(CLI_NAME) ./cmd/$(CLI_NAME)

install: build
	cp bin/$(CLI_NAME) $(GOPATH)/bin/$(CLI_NAME)

fmt:
	$(GOFUMPT) -l -w .
	$(GOIMPORTS) -local $(MODULE) -w .

lint:
	$(GOLANGCI_LINT) run ./...
	./scripts/lint-naming.sh

test:
	go test ./...

test-integration:
	go test -tags=integration ./internal/integration/...

ci: fmt lint test build

tools:
	GOBIN=$(TOOLS) go install mvdan.cc/gofumpt@latest
	GOBIN=$(TOOLS) go install golang.org/x/tools/cmd/goimports@latest
	GOBIN=$(TOOLS) go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

clean:
	rm -rf bin .tools
