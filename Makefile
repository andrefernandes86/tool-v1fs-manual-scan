BINARY   := v1fs-scanner
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.Version=$(VERSION)

.PHONY: all docker darwin darwin-arm64 linux windows darwin-app clean run

all: docker darwin linux windows

# ── Docker (original, unchanged) ──────────────────────────────────────────────
docker:
	docker build -t $(BINARY):latest .
	@echo "Docker image built: $(BINARY):latest"

# ── macOS (Apple Silicon) → apps/macos/ ───────────────────────────────────────
darwin-arm64:
	@mkdir -p apps/macos
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
	  go build -ldflags="$(LDFLAGS)" -o apps/macos/$(BINARY) .
	@echo "macOS (arm64) → apps/macos/$(BINARY)"

# ── macOS (Intel) → apps/macos/ ───────────────────────────────────────────────
darwin:
	@mkdir -p apps/macos
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 \
	  go build -ldflags="$(LDFLAGS)" -o apps/macos/$(BINARY) .
	@echo "macOS (amd64) → apps/macos/$(BINARY)"

# ── macOS .app bundle → apps/macos/V1FSScanner.app ───────────────────────────
darwin-app: darwin-arm64
	BINARY=$(BINARY) VERSION=$(VERSION) bash scripts/build-macos-app.sh
	@echo "macOS .app bundle → apps/macos/V1FSScanner.app"

# ── Linux x86-64 → apps/linux/ ────────────────────────────────────────────────
linux:
	@mkdir -p apps/linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	  go build -ldflags="$(LDFLAGS)" -o apps/linux/$(BINARY) .
	@echo "Linux (amd64) → apps/linux/$(BINARY)"

# ── Windows x86-64 → apps/windows/ ───────────────────────────────────────────
windows:
	@mkdir -p apps/windows
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
	  go build -ldflags="$(LDFLAGS) -H windowsgui" \
	  -o apps/windows/$(BINARY).exe .
	@echo "Windows (amd64) → apps/windows/$(BINARY).exe"

# ── Helpers ───────────────────────────────────────────────────────────────────
clean:
	rm -f apps/macos/$(BINARY) apps/linux/$(BINARY) apps/windows/$(BINARY).exe
	rm -rf apps/macos/V1FSScanner.app

run:
	go run .
