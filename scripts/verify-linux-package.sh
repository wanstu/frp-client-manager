#!/usr/bin/env bash
set -euo pipefail

shopt -s nullglob
debs=(dist/*-linux-amd64.deb)
if (( ${#debs[@]} != 1 )); then
  echo "expected exactly one Linux deb in dist/, found ${#debs[@]}" >&2
  exit 1
fi

deb="${debs[0]}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

dpkg-deb --extract "$deb" "$tmp"

required=(
  "usr/bin/frp-client-manager"
  "usr/share/applications/frp-client-manager.desktop"
  "usr/share/icons/hicolor/256x256/apps/frp-client-manager.png"
)
for path in "${required[@]}"; do
  if [[ ! -s "$tmp/$path" ]]; then
    echo "deb missing required file: /$path" >&2
    exit 1
  fi
done

desktop="$tmp/usr/share/applications/frp-client-manager.desktop"
grep -Fxq 'Exec=/usr/bin/frp-client-manager' "$desktop"
grep -Fxq 'TryExec=/usr/bin/frp-client-manager' "$desktop"
grep -Fxq 'Icon=frp-client-manager' "$desktop"
grep -Fxq 'Terminal=false' "$desktop"

echo "Linux deb launcher metadata verified: $deb"
