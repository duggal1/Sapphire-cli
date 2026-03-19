#!/bin/bash
set -e

echo "🔷 Building Sapphire CLI..."
echo ""

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "📦 Step 1/4: Cleaning module cache..."
go clean -modcache 2>/dev/null || true

echo "📦 Step 2/4: Downloading dependencies..."
env GOMODCACHE="${GOMODCACHE:-/tmp/sapphire-go-mod}" go mod download

echo "📦 Step 3/4: Tidying dependencies..."
env GOMODCACHE="${GOMODCACHE:-/tmp/sapphire-go-mod}" go mod tidy

echo "🔨 Step 4/4: Building binary..."
env GOCACHE="${GOCACHE:-/tmp/sapphire-go-build}" \
  GOMODCACHE="${GOMODCACHE:-/tmp/sapphire-go-mod}" \
  go build -buildvcs=false -ldflags="-s -w" -o ./sapphire .

echo ""
echo "✅ Build complete!"
echo ""
echo "📍 Binary location: $SCRIPT_DIR/sapphire"
echo ""
echo "🚀 Run with: ./sapphire"
echo ""

# Verify build
if [ -f "./sapphire" ]; then
    echo "📋 Version:"
    ./sapphire --version
    echo ""
    echo "✨ Ready to use!"
else
    echo "❌ Build failed - binary not found"
    exit 1
fi
