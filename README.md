<div align="center">

<img src="build/appicon.png" width="96" alt="Shelf icon">

# Shelf

A minimal, good-looking desktop library for your Steam, Epic Games and Ubisoft games.

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

**Library**

- Reads your Steam library from its local files. No login, no API key.
- Your Epic Games library alongside it, after a one-time sign-in (see [Epic Games](#epic-games)).
- Your Ubisoft library too, read from Ubisoft Connect once you've signed in there (see [Ubisoft](#ubisoft)).
- Poster grid with instant search, installed and favorites filters, a store filter and sorting. Stays smooth with a couple of hundred games.
- Hero banner for the game you played last, plus "Continue playing" and "Never played" shelves
- Detail sheet with playtime, last played, share of your library, store page and install folder
- Favorites and a random game picker
- **Collections and tags** of your own, to filter the library by (see [Collections and tags](#collections-and-tags))
- **Live library:** installs, uninstalls and playtime update automatically
- **Now playing:** a header indicator with a session timer
- **Activity:** a play-time heatmap and weekly stats, recorded while Shelf is running
- **Friends:** who is online and what they are playing, for the launchers that allow it (see [Friends](#friends))

**Epic Games**

- Install, update, verify, repair and uninstall games without leaving Shelf
- Launch through Proton with a separate prefix per game
- Cloud saves, per-game launch settings, and import of games you already have installed

**Ubisoft**

- Your whole Ubisoft library, installed or not, with Ubisoft's own poster and banner art
- Install, play and uninstall through Ubisoft Connect, which Shelf sets up and runs in its own Proton prefix
- Install progress, launch errors that say what went wrong, and a hint when the library is empty
- GE-Proton and the BattlEye runtime can be installed from Shelf with one button

**Everything else**

- Controller navigation with soft interface sounds, drawn with the buttons of your controller (Xbox, PlayStation or Nintendo)
- One **Integrations** page with a tab per store
- **Desktop integration:** application menu entries, `shelf://` links and a tray icon (see [Desktop integration](#desktop-integration))
- An optional **hardware monitor** with live CPU, memory, disk, network and GPU graphs (see [Hardware monitor](#hardware-monitor))
- A download queue, a disk usage view, and a log window
- Frameless glass UI, dark only

<p align="center">
  <img src="assets/library-epic.png" alt="Library showing Steam and Epic games, with update and install badges">
</p>

## Epic Games

Shelf drives [legendary](https://github.com/derrod/legendary), the same CLI Heroic uses, so it needs to be installed (Arch: `pacman -S legendary`). Shelf keeps its own legendary login, separate from any existing one.

1. Click the gear in the header, open **Integrations** and pick the **Epic Games** tab. Choose **Open Epic login**, sign in and paste the code Epic shows you.
2. Your Epic games appear in the library. Open one and press **Install**.
3. **Play** runs the game with Proton. Shelf finds Proton in Steam, `compatibilitytools.d` (GE-Proton) and Heroic's tools folder. Pick a version in the settings; by default the newest GE-Proton wins, then Proton Experimental.

<p align="center">
  <img src="assets/settings-epic.png" alt="Settings page, Epic Games section">
</p>

Games install to `~/Games/Shelf` (changeable), each with its own Proton prefix in `~/.local/share/shelf/prefixes`. Play time for Epic games is recorded while Shelf is running, since Epic doesn't provide it.

### Updates

Shelf checks for new versions at startup and every few hours, marks games that are behind and can install updates by itself. Both are switches in the Epic settings, and each game can opt in or out. Epic refuses to start an outdated game, so Shelf tells you when an update is needed. A game can also be allowed to launch outdated.

### Verify and repair

Check a game's files against Epic's manifest from its sheet. If anything is damaged or missing, repair downloads just those files again.

### Cloud saves

Newer saves are downloaded before a game starts and uploaded after it closes, or synced by hand from the sheet. Shelf finds the save folder inside the game's Proton prefix, and you can override it per game. A game has to be started once before its saves can sync. Only games that support cloud saves show the option.

### Per-game settings

Proton version, launch arguments, environment variables, MangoHud, GameMode, offline mode, whether outdated launches are allowed, and update and cloud save behavior. Each can follow the global setting.

### Existing installs

Games already installed by Heroic, legendary or the Epic Games Launcher are found and imported in place, with no download. Shelf only offers games that belong to your account.

### Games that can't be installed

Some Epic titles, such as Ubisoft Connect and EA app games, have to be installed through their own launcher, and Epic doesn't let other launchers download them. Shelf marks these in the game sheet instead of offering an install that can't work. If the game is also in your Ubisoft library, install it from there (see [Ubisoft](#ubisoft)).

## Ubisoft

Ubisoft has no command line tool like legendary, and its web login sits behind a bot check that only a real browser passes, so Shelf never asks for your Ubisoft password. Instead, [Ubisoft Connect](https://www.ubisoft.com/en-us/ubisoft-connect) itself is the account: Shelf installs it into a Proton prefix of its own, you sign in there once, and Shelf reads the library Connect records.

1. Click the gear, open **Integrations** and pick the **Ubisoft** tab. Press **Set up Ubisoft Connect**. Shelf downloads Ubisoft's installer and runs it quietly with Proton.
2. Press **Open Ubisoft Connect** and sign in, two-step verification included. Your games appear in Shelf on their own once Connect has loaded them. Until then the Ubisoft filter explains what is missing.
3. Open a game and press **Install**. Shelf hands the download to Connect and shows the game as installing, with how much has been written to disk (Connect reports no percentage). It counts as installed only when Connect marks it finished.
4. **Play** starts the game through Connect. If a game doesn't appear within two minutes, or Proton can't hand it over, Shelf says so and points at the game's log. **Uninstall** asks Connect to remove it; you confirm in Connect's window.

Games Ubisoft lists but that belong to Steam are shown in the library and left to Steam.

### GE-Proton

Ubisoft Connect behaves best with GE-Proton: black or frozen windows and slow game starts are the usual symptoms of other builds. If you aren't using it, the Ubisoft tab has an **Install GE-Proton** button. Shelf fetches the newest release from GitHub, checks its SHA-512, unpacks it into Steam's `compatibilitytools.d` (or Heroic's folder without Steam) and uses it from then on, unless you picked a build yourself. If Connect misbehaves after switching, **Reset** gives it a fresh prefix.

### BattlEye

Games that use BattlEye, such as Riders Republic, need Proton's BattlEye runtime to start. Shelf detects them from the game's folder, sets `PROTON_BATTLEYE_RUNTIME` when starting them (Epic games too), and refuses to launch them without it, saying what to do. The game's page offers **Install BattlEye runtime**, and so do the Epic and Ubisoft tabs. Steam's own "Proton BattlEye Runtime" is used if you have it. Otherwise Shelf downloads the same files Heroic and Lutris use, a 9 MB archive pinned to a known SHA-256, into `~/.local/share/shelf/runtimes`.

### Connect's window

Many setups draw Connect's own window black. **Fix a black Ubisoft Connect window** (on by default) uses software rendering for Connect's window only while you sign in and install. Playing a game restarts Connect normally, so games never use it.

## Downloads and storage

Installs, updates and repairs run one at a time. **Settings, Downloads** shows the queue: reorder it, send a game to the front, cancel entries or pause the queue. "Update all" queues every outdated game.

Ubisoft downloads are done by Connect, so they aren't in this queue; they show as installing on the game instead.

**Settings, Storage** shows how much space each Steam, Epic and Ubisoft game uses, free space per disk, and Proton prefixes, including leftovers from uninstalled games that you can delete. Ubisoft's games are installed inside one shared Connect prefix, so that is listed on its own with the games' space taken off, and is never offered as a leftover: **Reset** there deletes Connect's login and every Ubisoft game, and says so first.

<p align="center">
  <img src="assets/settings-downloads.png" width="49%" alt="Download queue">
  <img src="assets/settings-storage.png" width="49%" alt="Disk usage">
</p>

## Collections and tags

Open a game and use **Collections** and **Tags** in its sheet. A game can be in any number of collections (press **New collection** to make one on the spot) and carry any number of tags; type a tag and press Enter or a comma. The **Collections** button in the header narrows the library to one collection or tag, and works together with the store filter, search and the Favorites tab. **Settings, Collections** renames and deletes collections and removes a tag from every game.

Everything is saved in `~/.config/shelf/organizer.json`, separate from your favorites. Games are filed by store and id (`steam:620`, `epic:Fortnite`), so the same title on two stores can be filed differently.

## Friends

**More, Friends** lists your friends by what they are doing: playing, online, away, and offline (folded away). A friend's game is matched against your library, so you can open it straight away if you own it. The list refreshes every minute while the window is open and never in the background.

| Launcher | Friends | Why |
| --- | --- | --- |
| Steam | Yes | Through Steam's Web API |
| Epic Games | No | Epic only shares who is online over a private channel that other apps can't use |
| Ubisoft | No | Ubisoft Connect has no way for other apps to read your friends |

Epic and Ubisoft are listed in the window with the reason, so you know why they are empty.

**Steam setup.** Get a free [Web API key](https://steamcommunity.com/dev/apikey) (any domain name works, for example `localhost`), paste it in the Steam card in the Friends window and press Save. Shelf finds your account from the one Steam is signed in with on this computer. Your friends list has to be visible to you in Steam's privacy settings, which it is by default. The key is kept in `~/.config/shelf/friends.json`, readable only by you, and is only ever sent to Steam.

New launchers plug in by implementing one small `Provider` interface in `internal/friends`; the window draws their setup form from the fields they declare.

## Desktop integration

**Settings, Desktop.** Everything is off until you switch it on.

- **Application menu entries** add every installed game to your launcher, kept up to date as games come and go. Shelf only ever removes entries it wrote itself.
- **`shelf://` links** make `shelf://launch/steam:620` or `shelf://launch/epic:Fortnite` start a game from a browser, script or anything that opens URLs. Only installed games can be started this way, and a link can't do anything else. Menu entries use the same links. Only one Shelf runs at a time: a second start hands its link to the running one.
- **Tray icon** shows recently played games with a click to start them. It uses the StatusNotifier protocol, so GNOME needs the AppIndicator extension. **Keep running in the tray** makes closing the window hide it; quit from the tray menu. Both apply at the next start.

## Controller

Plug in a gamepad and press any button (the window needs focus). Focus moves smoothly between games, and the interface plays soft sounds as you go.

Shelf tells which kind of controller you have, by its USB vendor id, and shows the buttons it really has: coloured A/B/X/Y for Xbox, ✕ ○ □ △ for PlayStation, and Nintendo's layout with A and B swapped. Anything unknown is treated as an Xbox pad.

| Action | Xbox | PlayStation | Nintendo |
| --- | --- | --- | --- |
| Move | D-pad / left stick | D-pad / left stick | D-pad / left stick |
| Select | `A` | ✕ | `B` |
| Back, or close the open dialog | `B` | ○ | `A` |
| Favorite | `X` | □ | `Y` |
| Random game | `Y` | △ | `X` |
| Switch tabs | `LB` / `RB` | `L1` / `R1` | `L` / `R` |
| Settings | Menu | Options | `+` |
| Search | View | Create | `−` |
| Scroll | Right stick | Right stick | Right stick |

The sounds are Steam's own Deck UI sounds, played from your Steam install (nothing is copied or bundled). Without Steam, Shelf uses a small built-in set instead. Navigation, sounds and volume are under **Settings, Controller**. Dialogs and menus trap the focus, so the controller never wanders behind them.

## Hardware monitor

A task-manager-style page for seeing what your machine is doing while you play. It's off by default: turn on **Hardware monitor** under **Settings, Advanced**, which adds a button to the header (and `P` opens it). Nothing is sampled while the page is closed.

- **CPU:** utilization over the last minute, overall or per logical processor, with clock speed, temperature, processes, threads, load average and uptime
- **Memory:** usage, what is cached and could be reclaimed, free memory and swap
- **Disks:** active time, read and write speed, temperature and the space left on each mount
- **Network:** send and receive speed per adapter, with its link speed and addresses. Only real adapters are listed, not bridges or containers
- **GPUs:** utilization, video memory, temperature, power and clock for AMD (read from the kernel) and NVIDIA (through `nvidia-smi`). Intel shows its clock. A card that has powered itself down to save energy is shown as such and isn't woken to take a reading. An NVIDIA card that is awake stays awake while the page is open

Everything is read from `/proc` and `/sys`, once a second, so it needs no extra tools. Temperatures appear when the kernel has a sensor for them.

## Log window

**Settings, Advanced** turns on a log window: a console along the bottom of the window with live output from downloads, updates, game launches and cloud saves, including the commands that were run. Filter it, copy it or clear it. It opens by itself when a game fails to start. Each game's launch output is also written to `~/.local/share/shelf/logs`, whether or not the window is on.


## Keyboard

| Key | Action |
| --- | --- |
| `/` | Focus search |
| `Esc` | Clear search, or leave the settings page |
| `R` | Open a random game from the current view |

## Install

### Quick install

```bash
curl -fsSL https://raw.githubusercontent.com/0xby7eMe/shelf/main/install.sh | bash
```

The script installs the latest release for your user, with no root needed. It picks the right build for your system, checks its SHA-256, and adds Shelf to your application menu:

- **Arch, Manjaro, EndeavourOS and other Arch-based distros** get the native binary (`shelf-arch-x86_64.tar.gz`), built on Arch against the system GTK and WebKitGTK.
- **Ubuntu, Debian, Fedora and everything else** get the AppImage (`shelf-linux-x86_64.AppImage`), with GTK and WebKitGTK bundled.

Shelf ends up in `~/.local/bin/shelf`. Run the installer again to update. Options go after `bash -s --`:

```bash
curl -fsSL https://raw.githubusercontent.com/0xby7eMe/shelf/main/install.sh | bash -s -- --version v1.2.3
curl -fsSL https://raw.githubusercontent.com/0xby7eMe/shelf/main/install.sh | bash -s -- --appimage   # AppImage even on Arch
curl -fsSL https://raw.githubusercontent.com/0xby7eMe/shelf/main/install.sh | bash -s -- --uninstall  # keeps your settings
```

Only x86_64 Linux builds are published.

### Manual download

Everything is on the [releases page](https://github.com/0xby7eMe/shelf/releases).

**Arch:** `shelf-arch-x86_64.tar.gz` needs GTK 3 and WebKitGTK 4.1:

```bash
sudo pacman -S gtk3 webkit2gtk-4.1
tar -xzf shelf-arch-x86_64.tar.gz
install -Dm755 shelf ~/.local/bin/shelf
install -Dm644 appicon.png ~/.local/share/icons/hicolor/512x512/apps/io.github.0xby7eme.shelf.png
install -Dm644 shelf.desktop ~/.local/share/applications/shelf.desktop
```

**Ubuntu, Debian and others:** `shelf-linux-x86_64.AppImage` runs on Ubuntu 22.04 and later, Debian 12 and most other distros. Nothing else needs installing, except FUSE 2 (`sudo apt install libfuse2`) on systems that lack it:

```bash
chmod +x shelf-linux-x86_64.AppImage
./shelf-linux-x86_64.AppImage
```

### Requirements for the games

For Epic Games you also need legendary and a Proton build from Steam or ProtonUp-Qt (Arch: `sudo pacman -S legendary`). Ubisoft needs only a Proton build, and Shelf can install GE-Proton for you from the Ubisoft tab.

Controller support relies on WebKitGTK being built with gamepad support (libmanette), which is the case on Arch.

### From source

Requires Go 1.25+, Node 18+ and the [Wails CLI](https://wails.io), plus the development packages:

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
make icon     # render build/appicon.png from build/appicon.svg
make build    # production binary in build/bin/ (renders the icon first)
go test ./internal/...
```

## How it works

**Steam.** Shelf parses Steam's own files: `libraryfolders.vdf` for your library locations, the `appmanifest_*.acf` files for installed games and `localconfig.vdf` for playtime. A file watcher reloads the library when they change. Cover and hero art come from Steam's local library cache, with the Steam CDN as a fallback. "Now playing" looks for Steam's `SteamLaunch` wrapper process in `/proc`.

**Epic.** Shelf runs legendary with its own config folder and reads its files for the library and installed games. Games start as `legendary launch` with Proton as the wrapper, with `STEAM_COMPAT_DATA_PATH` pointing at the game's own prefix. Legendary exits right after starting the game, so "now playing" finds the game by the `-epicapp=<name>` argument on its processes. Downloads are one legendary process at a time, with progress read from its output.

**Ubisoft.** Shelf runs Ubisoft Connect through Proton in its own prefix and reads what Connect writes there: `cache/configuration/configurations` (every game it knows, with names and art) and `cache/ownership/<account id>` (the ids your account owns), both protobuf. The library is the games in both. Poster and banner art come from Ubisoft's launcher CDN. Install, uninstall and play are `uplay://` links handed to Connect through `proton run start`. Connect's registry marks the start of a download with an `Installs` key and its end with an `Uninstall` key, which is how Shelf tells installing from installed. Games are found running by their install folder in the process list.

Finished sessions of every store feed the activity heatmap.

| Data | Location |
| --- | --- |
| Favorites | `~/.config/shelf/favorites.json` |
| Play sessions | `~/.config/shelf/sessions.json` |
| Friends settings (Steam API key) | `~/.config/shelf/friends.json` |
| Epic settings | `~/.config/shelf/epic.json` and `epic-games.json` (per game) |
| Epic login and metadata | `~/.config/shelf/legendary` |
| Proton prefixes (Ubisoft's is `ubisoft-connect`) | `~/.local/share/shelf/prefixes` |
| BattlEye runtime (downloaded by Shelf) | `~/.local/share/shelf/runtimes/battleye_runtime` |
| GE-Proton (installed by Shelf) | `compatibilitytools.d` in your Steam folder |
| Game launch logs | `~/.local/share/shelf/logs` |
| Downloaded art | `~/.cache/shelf/covers` |

Built with Go, [Wails v2](https://wails.io), React, Tailwind CSS v4 and shadcn/ui.

## Releases

Pushing a `v*` tag builds both packages in GitHub Actions and attaches them to a release: the binary on an Arch container, and the AppImage on Ubuntu 22.04 so it runs on anything newer. Both builds render the app icon from `build/appicon.svg` first, so the icon in the window, the launcher and the AppImage always matches the SVG.

## Limitations

- Linux only for now. The Steam provider only knows Linux paths, including the Flatpak one.
- Only installed Steam games are listed, because Steam doesn't store names of uninstalled games locally. Epic shows your whole library.
- "Now playing" and session history work with native Steam, not the Flatpak version.
- Sessions are only recorded while Shelf is open, so the heatmap fills from first use. Playtime from Steam can lag until Steam writes its files, and Epic playtime counts only what Shelf saw.
- Epic needs legendary and a Proton build. Epic games that must be installed through Ubisoft Connect or the EA app can't be installed from the Epic side.
- Ubisoft can't be signed in from Shelf: you sign in inside Connect once, and Shelf reads the library from it. Connect's files are only read, never changed. The reading is built from Connect's current file format and tested against one account, so please open an issue if your library comes out empty.
- Ubisoft installs show activity and bytes written, not a percentage or time left, and can't be cancelled from Shelf. Playing or restarting Connect during a download is refused, since it would end the download.
- Ubisoft's play time is counted only while Shelf sees the game running, as with Epic.
- The download queue is kept in memory only, so it's empty after a restart. A cancelled download resumes where it stopped.
- Cloud saves rely on legendary finding the save folder in the game's Proton prefix. If it can't, set the save folder in the game's settings.
- Controller sounds depend on the webview allowing audio, which can need one click or key press after launch.
