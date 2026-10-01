VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
# Use an installed goreleaser, or fetch and run it on the fly.
GORELEASER ?= $(shell command -v goreleaser 2>/dev/null || echo "go run github.com/goreleaser/goreleaser/v2@latest")

.PHONY: build install uninstall test run snapshot clean

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/trnd .

# Installs to ~/.local/bin by default (on PATH on most Linux setups).
# Override with: make install PREFIX=/usr/local
PREFIX ?= $(HOME)/.local

install: build
	install -d $(PREFIX)/bin
	install -m 0755 bin/trnd $(PREFIX)/bin/trnd
	@echo "Installed $(PREFIX)/bin/trnd"

uninstall:
	rm -f $(PREFIX)/bin/trnd

test:
	go vet ./...
	go test ./...

run: build
	./bin/trnd

# Build all release artifacts locally into dist/ without publishing.
snapshot:
	$(GORELEASER) release --snapshot --clean

clean:
	rm -rf bin dist
