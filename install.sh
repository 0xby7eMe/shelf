#!/usr/bin/env bash
# Shelf installer.
#
#   curl -fsSL https://raw.githubusercontent.com/0xby7eMe/shelf/main/install.sh | bash
#
# Installs the latest release for your user (no root needed): the plain binary
# on Arch and its relatives, the AppImage on other Linux systems, and Shelf.app
# in ~/Applications on macOS. Windows has install.ps1.
#
# Options (pass them after `bash -s --`):
#   --version vX.Y.Z   install that release instead of the latest
#   --appimage         use the AppImage even on Arch (Linux only)
#   --uninstall        remove Shelf again (your settings are kept)
#   -h, --help         show this help
set -euo pipefail

REPO=0xby7eMe/shelf
BIN_DIR=${XDG_BIN_HOME:-$HOME/.local/bin}
DATA_DIR=${XDG_DATA_HOME:-$HOME/.local/share}
APP_FILE=$DATA_DIR/applications/shelf.desktop
# A unique icon name: icon themes may ship their own "shelf", and they win over hicolor.
ICON_FILE=$DATA_DIR/icons/hicolor/512x512/apps/io.github.0xby7eme.shelf.png
OLD_ICON_FILE=$DATA_DIR/icons/hicolor/512x512/apps/shelf.png
TARGET=$BIN_DIR/shelf
# macOS: an app bundle in the user's own Applications folder.
MAC_APPS=$HOME/Applications
MAC_APP=$MAC_APPS/Shelf.app

VERSION=latest
FORCE_APPIMAGE=0
UNINSTALL=0

# --- looks -----------------------------------------------------------------

if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != dumb ]; then
	TTY=1
	B=$'\e[1m' D=$'\e[2m' R=$'\e[0m'
	RED=$'\e[38;5;203m' GRN=$'\e[38;5;114m' YEL=$'\e[38;5;179m' BLU=$'\e[38;5;111m' MAG=$'\e[38;5;176m'
else
	TTY=0
	B= D= R= RED= GRN= YEL= BLU= MAG=
fi

SPIN_PID=
cleanup() {
	stop_spin
	[ "$TTY" = 1 ] && printf '\e[?25h'
	[ -n "${TMP:-}" ] && rm -rf "$TMP"
	return 0
}
trap cleanup EXIT
trap 'echo; exit 130' INT TERM

banner() {
	printf '\n'
	printf '  %s┏━┓╻ ╻┏━╸╻  ┏━╸%s\n' "$MAG" "$R"
	printf '  %s┗━┓┣━┫┣╸ ┃  ┣╸ %s   %sa minimal library for your games%s\n' "$BLU" "$R" "$D" "$R"
	printf '  %s┗━┛╹ ╹┗━╸┗━╸╹  %s\n\n' "$GRN" "$R"
}

# spin <message>: animate until stop_spin. Without a terminal it just prints.
spin() {
	if [ "$TTY" = 0 ]; then
		printf '  … %s\n' "$1"
		return
	fi
	printf '\e[?25l'
	(
		frames=(⠋ ⠙ ⠹ ⠸ ⠼ ⠴ ⠦ ⠧ ⠇ ⠏)
		i=0
		while :; do
			printf '\r\e[2K  %s%s%s %s' "$BLU" "${frames[i % 10]}" "$R" "$1"
			i=$((i + 1))
			sleep 0.08
		done
	) &
	SPIN_PID=$!
}

stop_spin() {
	[ -n "$SPIN_PID" ] || return 0
	kill "$SPIN_PID" 2>/dev/null || true
	wait "$SPIN_PID" 2>/dev/null || true
	SPIN_PID=
	printf '\r\e[2K\e[?25h'
}

ok() { stop_spin; printf '  %s✓%s %s\n' "$GRN" "$R" "$1"; }
note() { stop_spin; printf '  %s•%s %s\n' "$YEL" "$R" "$1"; }
die() {
	stop_spin
	printf '  %s✗%s %s\n\n' "$RED" "$R" "$1" >&2
	exit 1
}

# step <spinner message> <done message> <command...>
step() {
	local msg=$1 done=$2
	shift 2
	spin "$msg"
	"$@" || die "$msg failed"
	ok "$done"
}

# --- helpers ---------------------------------------------------------------

usage() { sed -n '2,/^set -/p' "$0" 2>/dev/null | sed '$d;s/^# \{0,1\}//'; }

need() { command -v "$1" >/dev/null 2>&1 || die "$1 is required but not installed"; }

# Arch, Manjaro, EndeavourOS, CachyOS and the like.
is_arch_like() {
	[ -r /etc/os-release ] || return 1
	local id id_like
	id=$(. /etc/os-release && echo "${ID:-}")
	id_like=$(. /etc/os-release && echo "${ID_LIKE:-}")
	case " $id $id_like " in *" arch "*) return 0 ;; esac
	return 1
}

release_url() { # asset
	if [ "$VERSION" = latest ]; then
		echo "https://github.com/$REPO/releases/latest/download/$1"
	else
		echo "https://github.com/$REPO/releases/download/$VERSION/$1"
	fi
}

download() { curl -fsSL --retry 3 --retry-delay 1 -o "$2" "$1"; }

is_macos() { [ "$(uname -s)" = Darwin ]; }

# sha256 of a file: sha256sum on Linux, shasum on macOS.
sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	else
		shasum -a 256 "$1" | awk '{print $1}'
	fi
}

verify() { # file checksum-file
	local want got
	want=$(awk '{print $1; exit}' "$2")
	got=$(sha256_of "$1")
	[ -n "$want" ] && [ "$want" = "$got" ]
}

# The tag the "latest" link points at, for the greeting at the end.
resolved_version() {
	if [ "$VERSION" != latest ]; then
		echo "$VERSION"
		return
	fi
	curl -fsSI -o /dev/null -w '%{url_effective}' -L "https://github.com/$REPO/releases/latest" 2>/dev/null |
		sed 's|.*/tag/||' || true
}

write_desktop_file() {
	mkdir -p "$(dirname "$APP_FILE")"
	cat >"$APP_FILE" <<EOF
[Desktop Entry]
Name=Shelf
Exec=$TARGET
Type=Application
Comment=A minimal library for your Steam, Epic and Ubisoft games
Icon=io.github.0xby7eme.shelf
Categories=Game;
Keywords=games;library;steam;epic;ubisoft;
Terminal=false
StartupWMClass=shelf
EOF
}

refresh_caches() {
	command -v update-desktop-database >/dev/null && update-desktop-database "$(dirname "$APP_FILE")" 2>/dev/null || true
	command -v gtk-update-icon-cache >/dev/null && gtk-update-icon-cache -qf "$DATA_DIR/icons/hicolor" 2>/dev/null || true
	return 0
}

# --- uninstall -------------------------------------------------------------

uninstall() {
	banner
	if is_macos; then
		step "Removing Shelf" "Removed Shelf" rm -rf "$MAC_APP"
		note "Your settings in ~/Library/Application Support/shelf were left alone"
		printf '\n'
		return
	fi
	step "Removing Shelf" "Removed Shelf" rm -f "$TARGET" "$APP_FILE" "$ICON_FILE" "$OLD_ICON_FILE"
	refresh_caches
	note "Your settings in ~/.config/shelf and ~/.local/share/shelf were left alone"
	printf '\n'
}

# --- install ---------------------------------------------------------------

install_arch() {
	local tar=shelf-arch-x86_64.tar.gz
	spin "Downloading Shelf for Arch"
	download "$(release_url $tar)" "$TMP/$tar" || die "couldn't download $tar from the $VERSION release"
	download "$(release_url $tar.sha256)" "$TMP/$tar.sha256" || die "couldn't download the checksum"
	ok "Downloaded"

	step "Verifying checksum" "Checksum matches" verify "$TMP/$tar" "$TMP/$tar.sha256"
	mkdir -p "$TMP/pkg"
	step "Unpacking" "Unpacked" tar -xzf "$TMP/$tar" -C "$TMP/pkg"
	[ -f "$TMP/pkg/shelf" ] && [ -f "$TMP/pkg/appicon.png" ] || die "the archive is missing the binary or icon"

	spin "Installing"
	install -Dm755 "$TMP/pkg/shelf" "$TARGET"
	install -Dm644 "$TMP/pkg/appicon.png" "$ICON_FILE"
	ok "Installed the binary to ${TARGET/#$HOME/\~}"

	if command -v pacman >/dev/null; then
		local missing=()
		for pkg in gtk3 webkit2gtk-4.1; do pacman -Qq "$pkg" >/dev/null 2>&1 || missing+=("$pkg"); done
		if [ ${#missing[@]} -gt 0 ]; then
			note "Shelf needs ${missing[*]}. Install it with: ${B}sudo pacman -S ${missing[*]}${R}"
		fi
	fi
}

install_appimage() {
	local img=shelf-linux-x86_64.AppImage
	spin "Downloading the Shelf AppImage"
	download "$(release_url $img)" "$TMP/$img" || die "couldn't download $img from the $VERSION release"
	download "$(release_url $img.sha256)" "$TMP/$img.sha256" || die "couldn't download the checksum"
	ok "Downloaded"

	step "Verifying checksum" "Checksum matches" verify "$TMP/$img" "$TMP/$img.sha256"

	spin "Installing"
	install -Dm755 "$TMP/$img" "$TARGET"
	ok "Installed the AppImage to ${TARGET/#$HOME/\~}"

	# The AppImage carries its icon; pull the same one out for the launcher entry.
	spin "Fetching the icon"
	if download "https://raw.githubusercontent.com/$REPO/${VERSION/latest/main}/build/appicon.png" "$TMP/appicon.png"; then
		install -Dm644 "$TMP/appicon.png" "$ICON_FILE"
		ok "Icon in place"
	else
		note "Couldn't fetch the icon; the launcher entry will have a generic one"
	fi

	if ! command -v fusermount >/dev/null && ! command -v fusermount3 >/dev/null; then
		note "AppImages need FUSE. On Ubuntu/Debian: ${B}sudo apt install libfuse2${R} (libfuse2t64 on 24.04)"
	fi
}

install_macos() {
	local zip=shelf-macos-universal.zip
	spin "Downloading Shelf for macOS"
	download "$(release_url $zip)" "$TMP/$zip" ||
		die "couldn't download $zip from the $VERSION release (macOS builds start with the first release after v0.6.0)"
	download "$(release_url $zip.sha256)" "$TMP/$zip.sha256" || die "couldn't download the checksum"
	ok "Downloaded"

	step "Verifying checksum" "Checksum matches" verify "$TMP/$zip" "$TMP/$zip.sha256"
	mkdir -p "$TMP/pkg"
	step "Unpacking" "Unpacked" ditto -x -k "$TMP/$zip" "$TMP/pkg"
	[ -d "$TMP/pkg/Shelf.app" ] || die "the archive has no Shelf.app"

	spin "Installing"
	mkdir -p "$MAC_APPS"
	rm -rf "$MAC_APP"
	ditto "$TMP/pkg/Shelf.app" "$MAC_APP"
	# Shelf isn't signed by Apple; without this, macOS would refuse the first start
	# for a copy that came through a browser. curl doesn't mark it, but be sure.
	xattr -dr com.apple.quarantine "$MAC_APP" 2>/dev/null || true
	ok "Installed Shelf.app to ${MAC_APP/#$HOME/\~}"

	if [ -d /Applications/Shelf.app ]; then
		note "There is another Shelf.app in /Applications. Remove it so you don't start the old one."
	fi
}

finish_macos() {
	local tag
	tag=$(resolved_version)
	printf '\n  %s%sShelf%s %sis ready.%s\n' "$B" "$GRN" "$R" "$B" "$R"
	[ -n "$tag" ] && printf '  %sversion %s%s\n' "$D" "$tag" "$R"
	printf '\n  Start it from Launchpad or Spotlight, or run %sopen -a Shelf%s.\n\n' "$B" "$R"
}

install_shelf() {
	banner
	need curl

	if is_macos; then
		need ditto
		need shasum
		TMP=$(mktemp -d)
		install_macos
		finish_macos
		return
	fi

	case $(uname -s) in
	MINGW* | MSYS* | CYGWIN*)
		die "on Windows, install from PowerShell: irm https://raw.githubusercontent.com/$REPO/main/install.ps1 | iex"
		;;
	esac

	need tar
	need sha256sum
	[ "$(uname -s)" = Linux ] || die "Shelf runs on Linux, macOS and Windows"
	case $(uname -m) in
	x86_64 | amd64) ;;
	*) die "only x86_64 builds are published (this is $(uname -m))" ;;
	esac

	TMP=$(mktemp -d)
	mkdir -p "$BIN_DIR"

	if is_arch_like && [ "$FORCE_APPIMAGE" = 0 ]; then
		note "Arch-based system: using the native binary"
		install_arch
	else
		note "Using the AppImage"
		install_appimage
	fi

	rm -f "$OLD_ICON_FILE" # from versions that used the plain name
	step "Adding Shelf to your application menu" "Launcher entry added" write_desktop_file
	refresh_caches

	local tag
	tag=$(resolved_version)
	printf '\n  %s%sShelf%s %sis ready.%s\n' "$B" "$GRN" "$R" "$B" "$R"
	[ -n "$tag" ] && printf '  %sversion %s%s\n' "$D" "$tag" "$R"
	printf '\n  Start it from your application menu, or run %sshelf%s.\n' "$B" "$R"
	case ":$PATH:" in
	*":$BIN_DIR:"*) ;;
	*) printf '  %s%s is not in your PATH yet. Add it to your shell profile to run %sshelf%s%s from a terminal.%s\n' "$YEL" "${BIN_DIR/#$HOME/\~}" "$B" "$R" "$YEL" "$R" ;;
	esac
	printf '\n'
}

# --- main ------------------------------------------------------------------

while [ $# -gt 0 ]; do
	case $1 in
	--version)
		VERSION=${2:?--version needs a value, e.g. v1.2.3}
		shift
		;;
	--appimage) FORCE_APPIMAGE=1 ;;
	--uninstall) UNINSTALL=1 ;;
	-h | --help)
		usage
		exit 0
		;;
	*) die "unknown option: $1" ;;
	esac
	shift
done

if [ "$UNINSTALL" = 1 ]; then uninstall; else install_shelf; fi
