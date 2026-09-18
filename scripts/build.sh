#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DESKTOP="$ROOT/cmd/frp-client-desktop"
ICON_SOURCE="$DESKTOP/assets/appicon.png"
BUILD_DIR="$DESKTOP/build"
APP_ICON="$BUILD_DIR/appicon.png"

mkdir -p "$BUILD_DIR"
cp "$ICON_SOURCE" "$APP_ICON"

cd "$ROOT"
node --check cmd/frp-client-desktop/frontend/app.js
node scripts/check-frontend.mjs
go test ./...
go vet ./...

cd "$DESKTOP"
case "$(uname -s)" in
  Linux)
    wails build -clean -tags webkit2_41
    ;;
  Darwin)
    wails build -clean -platform darwin/universal
    ;;
  *)
    echo "Unsupported Unix platform: $(uname -s)" >&2
    exit 1
    ;;
esac

echo
echo "Build complete:"
case "$(uname -s)" in
  Darwin) echo "$DESKTOP/build/bin/frp-client-manager.app" ;;
  *) echo "$DESKTOP/build/bin/frp-client-manager" ;;
esac
