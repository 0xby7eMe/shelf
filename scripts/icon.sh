#!/usr/bin/env bash
# Renders the app icon (build/appicon.png, which is embedded in the binary and
# used for the launcher entry and the AppImage) from build/appicon.svg, so the
# SVG stays the single source of truth.
#
#   scripts/icon.sh
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
SVG=$ROOT/build/appicon.svg
PNG=$ROOT/build/appicon.png

if command -v rsvg-convert >/dev/null; then
	rsvg-convert -w 1024 -h 1024 "$SVG" -o "$PNG"
elif command -v magick >/dev/null; then
	magick -background none -density 384 "$SVG" -resize 1024x1024 "$PNG"
else
	echo "icon.sh needs rsvg-convert (librsvg) or ImageMagick" >&2
	exit 1
fi

# Fail the build rather than ship a wrong icon.
size=$(file -b "$PNG")
case $size in
*"1024 x 1024"*) ;;
*) echo "unexpected icon size: $size" >&2; exit 1 ;;
esac
echo "icon rendered from build/appicon.svg"
