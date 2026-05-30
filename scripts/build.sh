#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_DIR"

VERSION="${1:-dev}"
LDFLAGS="-s -w -X main.version=${VERSION}"

echo "Building DevDash (version: ${VERSION})..."

echo "Building server (current platform)..."
cd server
go mod download
CGO_ENABLED=0 go build -ldflags="${LDFLAGS}" -trimpath -o devdash ./cmd/server
echo "Server built: server/devdash"

echo "Building agent (current platform)..."
CGO_ENABLED=0 go build -ldflags="${LDFLAGS}" -trimpath -o agent ./cmd/agent
echo "Agent built: server/agent"

cd "$PROJECT_DIR"

echo "Cross-compiling..."
mkdir -p dist

echo "  Linux amd64..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -trimpath -o dist/devdash-linux-amd64 ./server/cmd/server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="${LDFLAGS}" -trimpath -o dist/agent-linux-amd64 ./server/cmd/agent

echo "  Linux arm64..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -trimpath -o dist/devdash-linux-arm64 ./server/cmd/server
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="${LDFLAGS}" -trimpath -o dist/agent-linux-arm64 ./server/cmd/agent

echo "  Windows amd64..."
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -trimpath -o dist/devdash-windows-amd64.exe ./server/cmd/server
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="${LDFLAGS}" -trimpath -o dist/agent-windows-amd64.exe ./server/cmd/agent

echo "  Darwin amd64..."
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="${LDFLAGS}" -trimpath -o dist/devdash-darwin-amd64 ./server/cmd/server

echo "  Darwin arm64..."
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="${LDFLAGS}" -trimpath -o dist/devdash-darwin-arm64 ./server/cmd/server

echo ""
echo "Build complete! Binaries in dist/"
ls -lh dist/
