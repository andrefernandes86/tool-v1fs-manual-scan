#!/usr/bin/env bash
# Wraps the compiled macOS binary in a proper .app bundle.
# Called by: make darwin-app
# Expects:   apps/macos/v1fs-scanner  (built by make darwin-arm64 / darwin)
set -euo pipefail

BINARY="${BINARY:-v1fs-scanner}"
VERSION="${VERSION:-dev}"

SRC_BIN="apps/macos/${BINARY}"
APP="apps/macos/V1FSScanner.app"

if [ ! -f "$SRC_BIN" ]; then
  echo "Binary not found: $SRC_BIN"
  echo "Run 'make darwin-arm64' or 'make darwin' first."
  exit 1
fi

rm -rf "$APP"
mkdir -p "${APP}/Contents/MacOS"
mkdir -p "${APP}/Contents/Resources"

cp "$SRC_BIN" "${APP}/Contents/MacOS/V1FSScanner"
chmod +x "${APP}/Contents/MacOS/V1FSScanner"

cat > "${APP}/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key>         <string>V1FSScanner</string>
  <key>CFBundleIdentifier</key>         <string>com.trendmicro.v1fs-scanner</string>
  <key>CFBundleName</key>               <string>V1FS Scanner</string>
  <key>CFBundleDisplayName</key>        <string>V1FS Scanner</string>
  <key>CFBundleVersion</key>            <string>${VERSION}</string>
  <key>CFBundleShortVersionString</key> <string>${VERSION}</string>
  <key>CFBundlePackageType</key>        <string>APPL</string>
  <key>CFBundleSignature</key>          <string>????</string>
  <key>LSUIElement</key>                <false/>
  <key>NSHighResolutionCapable</key>    <true/>
  <key>LSMinimumSystemVersion</key>     <string>12.0</string>
</dict>
</plist>
EOF

echo "Bundle created: ${APP}"
echo ""
echo "To open:        open ${APP}"
echo "To distribute:  xattr -cr ${APP} && zip -r V1FSScanner.zip ${APP}"
