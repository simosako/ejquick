# EJQuick build rules. Release versions come only from Git tags; VERSION is
# available for local builds and defaults to a descriptive VCS revision.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/simosako/ejquick/internal/buildinfo.Version=$(VERSION)
GORELEASER ?= goreleaser

# Scratch binaries always go to tmp/ (never committed, see AGENTS.md).
.PHONY: build test test-race vet fmt fmt-check tidy-check bench bench-smoke clean release-check build-gui test-gui vet-gui

build:
	go build -ldflags "$(LDFLAGS)" -o tmp/ejquick ./cmd/ejquick
	go build -ldflags "$(LDFLAGS)" -o tmp/ejquick-build ./cmd/ejquick-build

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...

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

# GUI targets (design D35). They build the `gui`-tagged packages only and
# require a Qt 6 development toolchain reachable through pkg-config plus
# CGO; every target above stays Pure Go and Qt-free.
build-gui:
	CGO_ENABLED=1 go build -tags gui -ldflags "$(LDFLAGS)" -o tmp/ejquick-gui ./cmd/ejquick-gui

test-gui:
	CGO_ENABLED=1 go test -tags gui ./internal/gui

vet-gui:
	CGO_ENABLED=1 go vet -tags gui ./internal/gui ./cmd/ejquick-gui

clean:
	rm -rf dist tmp/dist tmp/ejquick tmp/ejquick-build tmp/ejquick-gui

# Validate the release configuration and build all release artifacts locally
# without publishing them.
release-check:
	$(GORELEASER) check
	$(GORELEASER) release --snapshot --clean
