<div align="center">

<img src="build/appicon.png" width="96" alt="Shelf icon">

# Shelf

A minimal, good-looking desktop library for your Steam games.

[![Build](https://github.com/0xby7eMe/shelf/actions/workflows/build.yml/badge.svg)](https://github.com/0xby7eMe/shelf/actions/workflows/build.yml)
[![Release](https://img.shields.io/github/v/release/0xby7eMe/shelf)](https://github.com/0xby7eMe/shelf/releases)

</div>

<p align="center">
  <img src="assets/library.png" alt="Library with hero banner, shelves and poster grid">
</p>

<p align="center">
  <img src="assets/detail.png" width="49%" alt="Game detail sheet">
  <img src="assets/favorites.png" width="49%" alt="Favorites filter">
</p>

## Features

- Reads your Steam library from its local files. No login, no API key.
- Poster grid with instant search, installed and favorites filters, and sorting
- Hero banner for the game you played last, plus "Continue playing" and "Never played" shelves
- Detail sheet with playtime, last played, share of your library, store page and install folder
- Favorites and a random game picker
- Launch games through Steam
- **Epic Games:** sign in, browse your library, install games and launch them through Proton (see below)
- **Live library:** installs, uninstalls and playtime update automatically
- **Now playing:** a header indicator with a session timer
- **Activity:** a play-time heatmap and weekly stats, recorded while Shelf is running
- Frameless glass UI, dark only

## Epic Games

Shelf drives [legendary](https://github.com/derrod/legendary), the same CLI Heroic uses, so it needs to be installed (Arch: `pacman -S legendary`). Shelf keeps its own legendary login, separate from any existing one.

1. Click the gamepad button in the header, choose **Open Epic login**, sign in and paste the code Epic shows you.
2. Your Epic games appear in the library. Open one and press **Install**.
3. **Play** runs the game with Proton, using one found in Steam, `compatibilitytools.d` (GE-Proton) or Heroic. Pick a specific version in the Epic dialog; by default the newest GE-Proton wins, then Proton Experimental.

Games install to `~/Games/Shelf` (changeable), each with its own Proton prefix in `~/.local/share/shelf/prefixes`. Launch logs are in `~/.local/share/shelf/logs`. Play time for Epic games is recorded while Shelf is running.

## Keyboard

| Key | Action |
| --- | --- |
| `/` | Focus search |
| `Esc` | Clear search |
| `R` | Open a random game from the current view |

## Install

### From a release

Download `shelf-linux-amd64.tar.gz` from the [releases page](https://github.com/0xby7eMe/shelf/releases), then:

```bash
tar -xzf shelf-linux-amd64.tar.gz
install -Dm755 shelf ~/.local/bin/shelf
install -Dm644 appicon.png ~/.local/share/icons/hicolor/512x512/apps/shelf.png
install -Dm644 shelf.desktop ~/.local/share/applications/shelf.desktop
```

The binary needs GTK 3 and WebKitGTK 4.1 at runtime:

```bash
# Arch
sudo pacman -S gtk3 webkit2gtk-4.1
# Debian/Ubuntu
sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0
```

### From source

Requires Go 1.21+, Node 18+ and the [Wails CLI](https://wails.io), plus the development packages:

```bash
# Arch
sudo pacman -S gtk3 webkit2gtk-4.1
# Debian/Ubuntu
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev
```

```bash
git clone https://github.com/0xby7eMe/shelf
cd shelf
cd frontend && npm install && cd ..
make install
```

`make install` builds the app and installs the binary, icon and launcher entry for your user.

## Development

```bash
make dev      # run with hot reload
make build    # production binary in build/bin/
```

## How it works

Shelf parses Steam's own files: `libraryfolders.vdf` for your library locations, the `appmanifest_*.acf` files for installed games and `localconfig.vdf` for playtime. A file watcher reloads the library when they change. Cover and hero art come from Steam's local library cache, with the Steam CDN as a fallback.

"Now playing" looks for Steam's `SteamLaunch` wrapper process in `/proc`. Finished sessions feed the activity heatmap.

| Data | Location |
| --- | --- |
| Favorites | `~/.config/shelf/favorites.json` |
| Play sessions | `~/.config/shelf/sessions.json` |
| Downloaded art | `~/.cache/shelf/covers` |

Built with Go, [Wails v2](https://wails.io), React, Tailwind CSS v4 and shadcn/ui.

## Limitations

- Linux only for now. The Steam provider only knows Linux paths, including the Flatpak one.
- Only installed games are listed, because Steam doesn't store names of uninstalled games locally.
- "Now playing" and session history work with native Steam, not the Flatpak version.
- Sessions are only recorded while Shelf is open, so the heatmap fills from first use. Playtime from Steam can lag until Steam writes its files.