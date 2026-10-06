<div align="center">

<img src="build/appicon.png" width="96" alt="Shelf icon">

# Shelf

A minimal, good-looking desktop library for your Steam games.

</div>

<p align="center">
  <img src="assets/library.png" alt="Library with hero banner and poster grid">
</p>

<p align="center">
  <img src="assets/detail.png" width="49%" alt="Game detail sheet">
  <img src="assets/favorites.png" width="49%" alt="Favorites filter">
</p>

## Features

- Reads your Steam library straight from its local files. No login, no API key.
- Poster grid with instant search, installed and favorites filters, and sorting
- Hero banner for the game you played last
- Detail sheet with playtime, last played, share of your library, store page and install folder
- Favorites, saved locally
- Random game picker
- Launch games through Steam
- Frameless glass UI, dark only
- Session history with a play-time heatmap and weekly stats (recorded while Shelf is running)

## Keyboard

| Key | Action |
| --- | --- |
| `/` | Focus search |
| `Esc` | Clear search |
| `R` | Open a random game from the current view |

## Install

Requires Linux, Steam, Go 1.21+, Node 18+, the [Wails CLI](https://wails.io) and, on Linux, GTK and WebKit:

```bash
# Arch
sudo pacman -S gtk3 webkit2gtk-4.1
# Debian/Ubuntu
sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev
```

Then:

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

Shelf parses Steam's own files: `libraryfolders.vdf` for your library locations, the `appmanifest_*.acf` files for installed games, and `localconfig.vdf` for playtime. Cover and hero art come from Steam's local library cache, with the Steam CDN as a fallback. Downloads are cached in `~/.cache/shelf/covers`. Favorites live in `~/.config/shelf/favorites.json`.

Built with Go, [Wails v2](https://wails.io), React, Tailwind CSS v4 and shadcn/ui.

## Limitations

- Linux only for now. The Steam provider only knows Linux paths, including the Flatpak one.
- Only installed games are listed. Steam doesn't store names of uninstalled games locally.
- Playtime is read from Steam's files and can lag until Steam writes them.