# Linux needs the WebKitGTK 4.1 tag; macOS uses the system WebKit.
ifeq ($(shell uname -s),Darwin)
TAGS :=
PLATFORM := darwin/universal
else
TAGS := webkit2_41
PLATFORM :=
endif
# Releases are built with the tag name; anything else is a "dev" build that never checks for updates.
VERSION ?= dev
LDFLAGS := -X main.version=$(VERSION)

.PHONY: dev build icon install

dev:
	wails dev $(if $(TAGS),-tags $(TAGS))

icon:
	scripts/icon.sh

build: icon
	wails build $(if $(TAGS),-tags $(TAGS)) $(if $(PLATFORM),-platform $(PLATFORM)) -ldflags "$(LDFLAGS)"

install: build
ifeq ($(shell uname -s),Darwin)
	rm -rf /Applications/Shelf.app
	cp -R build/bin/Shelf.app /Applications/Shelf.app
else
	sudo install -Dm755 build/bin/shelf /usr/bin/shelf
	sudo install -Dm644 build/appicon.png /usr/share/icons/hicolor/1024x1024/apps/io.github.0xby7eme.shelf.png
	sudo install -Dm644 build/linux/shelf.desktop /usr/share/applications/shelf.desktop
	sudo gtk-update-icon-cache -f /usr/share/icons/hicolor
	sudo update-desktop-database /usr/share/applications
endif
