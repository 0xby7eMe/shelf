TAGS := webkit2_41

.PHONY: dev build install

dev:
	wails dev -tags $(TAGS)

build:
	wails build -tags $(TAGS)

install: build
	install -Dm755 build/bin/shelf $(HOME)/.local/bin/shelf
	install -Dm644 build/appicon.png $(HOME)/.local/share/icons/hicolor/512x512/apps/shelf.png
	install -Dm644 build/linux/shelf.desktop $(HOME)/.local/share/applications/shelf.desktop
	-update-desktop-database $(HOME)/.local/share/applications