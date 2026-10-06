TAGS := webkit2_41

.PHONY: dev build install

dev:
	wails dev -tags $(TAGS)

build:
	wails build -tags $(TAGS)

install: build
	sudo install -Dm755 build/bin/shelf /usr/bin/shelf
	sudo install -Dm644 build/appicon.png /usr/share/icons/hicolor/1024x1024/apps/shelf.png
	sudo install -Dm644 build/linux/shelf.desktop /usr/share/applications/shelf.desktop
	sudo gtk-update-icon-cache -f /usr/share/icons/hicolor
	sudo update-desktop-database /usr/share/applications