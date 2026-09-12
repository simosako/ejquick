# EJQuick build rules. Override the version on the command line:
#   make build VERSION=v0.1.0
#   make release VERSION=v0.1.0
VERSION ?= dev
LDFLAGS := -X main.version=$(VERSION)

# Release targets from the design document. Archives include LICENSE and
# THIRD_PARTY_NOTICES as required for redistribution.
TARGETS := \
	linux/amd64 \
	linux/arm64 \
	windows/amd64 \
	windows/arm64 \
	darwin/amd64 \
	darwin/arm64

# Scratch binaries always go to tmp/ (never committed, see AGENTS.md).
.PHONY: build test test-race vet fmt fmt-check tidy-check bench bench-smoke clean release

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

clean:
	rm -rf dist tmp/ejquick tmp/ejquick-build

# release builds cross-compiled static binaries into dist/ archives.
# VERSION is taken from the invoking git tag by convention.
release:
	@for target in $(TARGETS); do \
		os=$${target%/*}; arch=$${target#*/}; \
		ext=""; archive="tar.gz"; \
		if [ "$$os" = "windows" ]; then ext=".exe"; archive="zip"; fi; \
		name="ejquick_$(VERSION)_$${os}_$${arch}"; \
		echo "== $$name"; \
		mkdir -p dist/$$name; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath -ldflags "$(LDFLAGS) -s -w" \
			-o dist/$$name/ejquick$$ext ./cmd/ejquick && \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath -ldflags "$(LDFLAGS) -s -w" \
			-o dist/$$name/ejquick-build$$ext ./cmd/ejquick-build || exit 1; \
		cp LICENSE THIRD_PARTY_NOTICES README.md dist/$$name/; \
		if [ "$$archive" = "tar.gz" ]; then \
			tar -C dist -czf dist/$$name.tar.gz $$name; \
		else \
			go run ./tools/mkzip dist/$$name.zip dist/$$name; \
		fi; \
		rm -rf dist/$$name; \
	done
	@ls -la dist/
