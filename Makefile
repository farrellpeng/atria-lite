.PHONY: build build-lite test vet lint install release clean

VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build:
	go build -ldflags "$(LDFLAGS)" -o atria .

build-lite:
	VERSION="$(VERSION)" COMMIT="$(COMMIT)" DATE="$(DATE)" ./scripts/build-atria-lite.sh

test:
	go test ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

install:
	go install -ldflags "$(LDFLAGS)" .

release:
	goreleaser release --clean

clean:
	rm -f atria
	rm -rf dist/
