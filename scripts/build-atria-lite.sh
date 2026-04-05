#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${ROOT_DIR}"

if [[ -x /usr/local/go/bin/go ]]; then
  GO_BIN=/usr/local/go/bin/go
elif command -v go >/dev/null 2>&1; then
  GO_BIN="$(command -v go)"
else
  echo "error: no usable go binary found" >&2
  exit 1
fi

VERSION="${VERSION:-dev}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo unknown)}"
DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
LDFLAGS="-X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}"

: "${GOPATH:=/tmp/atria-lite-gopath}"
: "${GOCACHE:=/tmp/atria-lite-gocache}"
mkdir -p "${GOPATH}" "${GOCACHE}" dist

env GOPATH="${GOPATH}" GOCACHE="${GOCACHE}" \
  "${GO_BIN}" build -mod=readonly -ldflags "${LDFLAGS}" -o dist/atria-lite ./cmd/atria-lite

echo "built ./dist/atria-lite"
