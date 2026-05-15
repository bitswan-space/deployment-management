#!/bin/sh
set -e

# Live dev mode: /app is mounted read-write; watch for file changes and auto-rebuild using Air
if [ "$BITSWAN_AUTOMATION_STAGE" = "live-dev" ]; then
  cd /app
  cp /deps/go.mod /deps/go.sum .
  echo "Downloading Go dependencies..."
  go mod download
  echo "Starting in live-dev mode with auto-rebuild (Air)..."
  exec air -c /etc/air.toml
fi

# Production mode: /app is read-only, so copy source to writable location, build once and run
cp -r /app /tmp/app
cd /tmp/app
cp /deps/go.mod /deps/go.sum .
echo "Downloading Go dependencies..."
go mod download
echo "Building Go server..."
CGO_ENABLED=0 go build -o /tmp/server .
echo "Starting server..."
exec /tmp/server
