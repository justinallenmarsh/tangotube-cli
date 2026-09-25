VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
PKG     := github.com/justinallenmarsh/tangotube-cli/internal/commands
LDFLAGS := -s -w -X $(PKG).Version=$(VERSION) -X $(PKG).Commit=$(COMMIT) -X $(PKG).Date=$(DATE)

.DEFAULT_GOAL := check
.PHONY: check fmt fmt-check vet test build smoke brand clean

check: fmt-check vet test

fmt:
	gofmt -w .

fmt-check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

vet:
	go vet ./...

test:
	go test ./...

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/tt ./cmd/tt

smoke: build
	bats e2e/smoke.bats

brand:
	go run ./tools/brandgif -o docs/images/tt.gif

clean:
	rm -rf bin dist
