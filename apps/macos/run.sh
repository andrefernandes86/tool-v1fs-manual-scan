#!/usr/bin/env bash
# V1FS Scanner - macOS launcher
# Builds the binary on first run (or when source changes), then opens the app.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BIN="$SCRIPT_DIR/v1fs-scanner"
PORT="${PORT:-8080}"

# Check Go
if ! command -v go &>/dev/null; then
  echo ""
  echo "  Go is required but not installed."
  echo "  Install it with Homebrew:  brew install go"
  echo "  Or download from:          https://go.dev/dl/"
  echo ""
  exit 1
fi

# Detect architecture
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
  GOARCH="arm64"
else
  GOARCH="amd64"
fi

# Build if binary is missing or source is newer
NEEDS_BUILD=0
if [ ! -f "$BIN" ]; then
  NEEDS_BUILD=1
else
  for f in "$ROOT/main.go" "$ROOT/go.mod" "$ROOT/web.go" \
            "$ROOT/platform_darwin.go" "$ROOT/internal/api/handler.go" \
            "$ROOT/internal/scanner/store.go" \
            "$ROOT/internal/api/eicar.go"; do
    if [ -f "$f" ] && [ "$f" -nt "$BIN" ]; then
      NEEDS_BUILD=1
      break
    fi
  done
fi

if [ "$NEEDS_BUILD" -eq 1 ]; then
  echo "Building V1FS Scanner for macOS ($ARCH)..."
  (cd "$ROOT" && CGO_ENABLED=0 GOOS=darwin GOARCH="$GOARCH" \
    go build -ldflags="-s -w" -o "$BIN" .)
  echo "Build complete."
fi

# Stop any stale instance on the same port
lsof -ti ":${PORT}" | xargs kill -9 2>/dev/null || true

# Launch - platformInit inside the binary opens the browser automatically
echo "Starting V1FS Scanner on port ${PORT}..."
exec "$BIN"
