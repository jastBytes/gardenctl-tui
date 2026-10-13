BIN_DIR         := $(CURDIR)/bin
GOLANGCI_LINT   := $(BIN_DIR)/golangci-lint
GOLANGCI_VER    := v2.14.0
GOVULNCHECK_VER := v1.9.0
VERSION         ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS         := -s -w -X main.version=$(VERSION)

.PHONY: all build test vet lint fmt tidy vuln clean snapshot demo-gif

all: build vet test lint

build:
	go build -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/gtui ./cmd/gtui

test:
	go test ./...

vet:
	go vet ./...

lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run ./...

fmt: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt ./...

tidy:
	go mod tidy

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VER) ./...

$(GOLANGCI_LINT):
	GOBIN=$(BIN_DIR) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VER)

snapshot:
	goreleaser release --snapshot --clean --skip=publish

demo-gif: build
	PATH=$(BIN_DIR):$$PATH vhs docs/demo.tape

clean:
	rm -rf $(BIN_DIR) dist
