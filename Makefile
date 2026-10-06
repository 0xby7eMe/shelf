TAGS := webkit2_41

.PHONY: dev build

dev:
	wails dev -tags $(TAGS)

build:
	wails build -tags $(TAGS)