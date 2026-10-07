#!/usr/bin/env bash
# Builds a self-contained AppImage with GTK 3 and WebKitGTK bundled, so it runs
# on distros that don't have them installed.
#
#   scripts/build-appimage.sh <shelf binary> <output.AppImage>
#
# Needs the GTK 3 and WebKitGTK 4.1 development packages (the libraries it
# bundles are the ones installed on the machine it runs on), plus ImageMagick.
# Build on the oldest distro you want to support: an AppImage runs anywhere
# with the same or a newer glibc.
set -euo pipefail

BIN=${1:?usage: build-appimage.sh <shelf binary> <output.AppImage>}
OUT=${2:?usage: build-appimage.sh <shelf binary> <output.AppImage>}

ROOT=$(cd "$(dirname "$0")/.." && pwd)
WORK=${WORK_DIR:-$ROOT/build/appimage}
TOOLS=$WORK/tools
APPDIR=$WORK/AppDir
export ARCH=${ARCH:-x86_64}

mkdir -p "$TOOLS"
fetch() { # url file
	[ -x "$TOOLS/$2" ] || { curl -fsSL --retry 3 -o "$TOOLS/$2" "$1" && chmod +x "$TOOLS/$2"; }
}
fetch "https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-$ARCH.AppImage" "linuxdeploy-$ARCH.AppImage"
fetch "https://raw.githubusercontent.com/linuxdeploy/linuxdeploy-plugin-gtk/master/linuxdeploy-plugin-gtk.sh" "linuxdeploy-plugin-gtk.sh"
fetch "https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-$ARCH.AppImage" "appimagetool-$ARCH.AppImage"

# CI machines have no FUSE; the tools unpack themselves and run instead.
export APPIMAGE_EXTRACT_AND_RUN=1
export PATH="$TOOLS:$PATH"
export DEPLOY_GTK_VERSION=3

rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin"
install -m755 "$BIN" "$APPDIR/usr/bin/shelf"

# Launcher entry and icon, as AppImage wants them (relative Exec, sized icon).
sed -e 's|^Exec=.*|Exec=shelf|' "$ROOT/build/linux/shelf.desktop" > "$WORK/shelf.desktop"
if command -v magick >/dev/null; then IM=(magick); else IM=(convert); fi
"${IM[@]}" "$ROOT/build/appicon.png" -resize 512x512 "$WORK/shelf.png"

# WebKit starts its helper processes from a path fixed at compile time, which
# won't exist on the user's machine. Bundle them where the same path, made
# relative, will find them (see the patch below).
LIBEXEC=$(dirname "$(find /usr/lib* /usr/libexec -type f -name WebKitWebProcess -path '*webkit2gtk-4.1*' 2>/dev/null | head -1)")
[ -x "$LIBEXEC/WebKitWebProcess" ] || { echo "WebKitGTK 4.1 helper processes not found" >&2; exit 1; }
REL=${LIBEXEC#/usr/}
mkdir -p "$APPDIR/usr/$REL"
for helper in WebKitWebProcess WebKitNetworkProcess WebKitGPUProcess; do
	[ -x "$LIBEXEC/$helper" ] && install -m755 "$LIBEXEC/$helper" "$APPDIR/usr/$REL/$helper"
done
[ -d "$LIBEXEC/injected-bundle" ] && cp -r "$LIBEXEC/injected-bundle" "$APPDIR/usr/$REL/"

ARGS=(--appdir "$APPDIR" --desktop-file "$WORK/shelf.desktop" --icon-file "$WORK/shelf.png" --plugin gtk)
for exe in "$APPDIR/usr/bin/shelf" "$APPDIR/usr/$REL"/WebKit*Process; do ARGS+=(--executable "$exe"); done
for lib in "$APPDIR/usr/$REL"/injected-bundle/*.so; do [ -f "$lib" ] && ARGS+=(--library "$lib"); done
linuxdeploy-$ARCH.AppImage "${ARGS[@]}"

# Make WebKit's helper path relative: "/usr" -> "././" keeps the length, so
# "/usr/lib/<arch>/webkit2gtk-4.1/WebKitWebProcess" becomes
# "././lib/<arch>/webkit2gtk-4.1/WebKitWebProcess", which resolves inside the
# AppImage because Shelf changes into usr/ when it starts (library.EnterAppImage).
patched=0
while IFS= read -r so; do
	sed -i -e 's|/usr|././|g' "$so"
	patched=$((patched + 1))
done < <(find "$APPDIR" -type f -name 'libwebkit2gtk-4.1.so*')
[ "$patched" -gt 0 ] || { echo "libwebkit2gtk was not bundled" >&2; exit 1; }

mkdir -p "$(dirname "$OUT")"
appimagetool-$ARCH.AppImage "$APPDIR" "$OUT"
echo "built $OUT ($(du -h "$OUT" | cut -f1))"
