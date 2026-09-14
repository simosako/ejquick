# EJQuick build rules. Release versions come only from Git tags; VERSION is
# available for local builds and defaults to a descriptive VCS revision.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/simosako/ejquick/internal/buildinfo.Version=$(VERSION)
GORELEASER ?= goreleaser
QT_PREFIX ?= /usr
QT_PKG_CONFIG_PATH ?= $(QT_PREFIX)/lib/pkgconfig
GUI_ENV := CGO_ENABLED=1 PKG_CONFIG_PATH="$(QT_PKG_CONFIG_PATH):$${PKG_CONFIG_PATH}"

# Scratch binaries always go to tmp/ (never committed, see AGENTS.md).
.PHONY: build build-gui test test-gui test-race vet vet-gui fmt fmt-check tidy-check bench bench-smoke clean release-check

build:
	go build -ldflags "$(LDFLAGS)" -o tmp/ejquick ./cmd/ejquick
	go build -ldflags "$(LDFLAGS)" -o tmp/ejquick-build ./cmd/ejquick-build

build-gui:
	$(GUI_ENV) go build -tags gui -ldflags "$(LDFLAGS)" -o tmp/ejquick-gui ./cmd/ejquick-gui

test:
	go test ./...

test-gui:
	$(GUI_ENV) go test -tags gui ./cmd/ejquick-gui ./internal/gui/...

test-race:
	go test -race ./...

vet:
	go vet ./...

vet-gui:
	$(GUI_ENV) go vet -tags gui ./cmd/ejquick-gui ./internal/gui/...

fmt:
	gofmt -w bench cmd internal tools

fmt-check:
	@files="$$(gofmt -l bench cmd internal tools)"; \
		test -z "$$files" || { echo "gofmt required for:"; echo "$$files"; exit 1; }

tidy-check:
	go mod tidy -diff

bench:
	go test -run '^$$' -bench . -benchmem ./internal/builder ./internal/search

bench-smoke:
	go test -run '^$$' -bench . -benchmem -benchtime=1x ./internal/builder ./internal/search

clean:
	rm -rf dist tmp/dist tmp/ejquick tmp/ejquick-build tmp/ejquick-gui

# Validate the release configuration and build all release artifacts locally
# without publishing them.
release-check:
	$(GORELEASER) check
	$(GORELEASER) release --snapshot --clean
