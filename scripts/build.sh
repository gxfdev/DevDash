#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "Building DevDash..."

# Build server
echo "Building server..."
cd server
go mod download
go build -ldflags="-s -w" -o devdash ./cmd/server
echo "Server built: server/devdash"

# Build agent
echo "Building agent..."
go build -ldflags="-s -w" -o agent ./cmd/agent
echo "Agent built: server/agent"

cd ..

# Cross-compile for multiple platforms
echo "Cross-compiling..."

# Linux amd64
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/devdash-linux-amd64 ./server/cmd/server
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o dist/agent-linux-amd64 ./server/cmd/agent

# Linux arm64
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/devdash-linux-arm64 ./server/cmd/server
GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o dist/agent-linux-arm64 ./server/cmd/agent

# Windows amd64
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/devdash-windows-amd64.exe ./server/cmd/server
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o dist/agent-windows-amd64.exe ./server/cmd/agent

# Darwin amd64
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o dist/devdash-darwin-amd64 ./server/cmd/server

# Darwin arm64
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o dist/devdash-darwin-arm64 ./server/cmd/server

echo ""
echo "Build complete! Binaries in dist/"
ls -la dist/