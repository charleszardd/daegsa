# DAEGSA Makefile (§5, §15)

VERSION ?= v0.1.0-dev
COMMIT ?= $(shell git rev-parse --short=12 HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo "unknown")

PKG_CLI := github.com/charleszardd/daegsa/internal/cli
PKG_REPORT := github.com/charleszardd/daegsa/internal/report

LDFLAGS := -s -w \
	-X $(PKG_CLI).Version=$(VERSION) \
	-X $(PKG_CLI).Commit=$(COMMIT) \
	-X $(PKG_CLI).BuildDate=$(BUILD_DATE) \
	-X $(PKG_REPORT).DefaultDaegsaVersion=$(VERSION) \
	-X $(PKG_REPORT).DefaultCommit=$(COMMIT) \
	-X $(PKG_REPORT).DefaultBuildDate=$(BUILD_DATE)

.PHONY: all build build-gui gui test test-race vet fmt fmt-check doctor self-test cross-build package sbom release clean

all: build

build:
	@mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/daegsa ./cmd/daegsa

build-gui:
	@mkdir -p bin
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/daegsa-gui ./cmd/daegsa-gui

gui:
	go run ./cmd/daegsa-gui

test:
	go test -count=1 ./...

test-race:
	go test -race -count=1 ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Unformatted files found:" && gofmt -l . && exit 1)

doctor:
	go run ./cmd/daegsa doctor

self-test:
	go run ./cmd/daegsa self-test

cross-build:
	go run scripts/package.go -version $(VERSION) -commit $(COMMIT) -build-date $(BUILD_DATE)

package:
	go run scripts/package.go -version $(VERSION) -commit $(COMMIT) -build-date $(BUILD_DATE)

sbom:
	go run scripts/sbom.go

release: clean package sbom
	@echo "Release build and packaging complete in dist/"

clean:
	rm -rf bin/ dist/
