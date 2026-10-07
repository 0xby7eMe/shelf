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
- **Gamepad:** navigate, select, favorite and launch with a controller, with soft interface sounds
- **Settings page:** Epic account and options, download queue, disk usage and controller settings in one place
- **Live library:** installs, uninstalls and playtime update automatically
- **Now playing:** a header indicator with a session timer
- **Activity:** a play-time heatmap and weekly stats, recorded while Shelf is running
- Frameless glass UI, dark only

## Epic Games

Shelf drives [legendary](https://github.com/derrod/legendary), the same CLI Heroic uses, so it needs to be installed (Arch: `pacman -S legendary`). Shelf keeps its own legendary login, separate from any existing one.

1. Click the gamepad button in the header, choose **Open Epic login**, sign in and paste the code Epic shows you.
2. Your Epic games appear in the library. Open one and press **Install**.
3. **Play** runs the game with Proton, using one found in Steam, `compatibilitytools.d` (GE-Proton) or Heroic. Pick a specific version in the Epic dialog; by default the newest GE-Proton wins, then Proton Experimental.

Beyond install and play:

- **Updates:** Shelf checks for new versions at startup and every few hours, marks games that are behind, and can install updates by itself. Both are switches in the Epic dialog, and each game can opt in or out. Epic refuses to start an outdated game, so a game can also be allowed to launch outdated.
- **Verify and repair:** check a game's files against Epic's manifest from its sheet, and repair anything damaged.
- **Cloud saves:** newer saves are downloaded before a game starts and uploaded after it closes. Shelf finds the save folder inside the game's Proton prefix. A game has to be started once before its saves can sync.
- **Per-game settings:** Proton version, launch arguments, environment variables, MangoHud, GameMode, offline mode, update and cloud save behavior.
- **Existing installs:** games already installed by Heroic, legendary or the Epic Games Launcher are found and imported in place, with no download.

Games install to `~/Games/Shelf` (changeable), each with its own Proton prefix in `~/.local/share/shelf/prefixes`. Launch logs are in `~/.local/share/shelf/logs`. Play time for Epic games is recorded while Shelf is running.

## Log window

Settings, Advanced turns on a log window: a console along the bottom of the window with live output from downloads, updates, game launches and cloud saves, filterable and copyable. It opens by itself when a game fails to start. Each game's launch output is also written to `~/.local/share/shelf/logs`.

## Controller

Plug in a gamepad and press any button (the window needs focus). The D-pad or left stick moves, **A** selects, **B** goes back or closes, **X** favorites, **Y** picks a random game, **LB/RB** switch tabs, **Start** opens settings and the right stick scrolls. Sounds and navigation can be turned off under Settings, Controller.

## Downloads and storage

Installs, updates and repairs run one at a time. Settings, Downloads shows the queue: reorder it, send a game to the front, cancel entries or pause the queue. Settings, Storage shows how much space each Steam and Epic game uses, free space per disk, and Proton prefixes, including leftovers from uninstalled games that you can delete.

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