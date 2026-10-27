BINARY=safe-past
DAEMON=safe-past-daemon
VERSION?=dev
LDFLAGS=-s -w -X main.version=$(VERSION)
# Windows tray daemon: hide the console window.
WIN_GUI_LDFLAGS=-s -w -H=windowsgui -X main.version=$(VERSION)

.PHONY: test test-pure build build-daemon build-all install clean

test:
	go test ./...

# No display/system deps needed: pure engine + config + hotkey parsing.
test-pure:
	go test ./internal/... ./cmd/safe-past/...

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/safe-past

# Linux needs libX11-dev for the global hotkey (apt install libX11-dev).
# macOS/Windows build on their runners or via GoReleaser (phase 3).
build-daemon:
	go build -ldflags "$(LDFLAGS)" -o bin/$(DAEMON) ./cmd/safe-past-daemon

build-all:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe ./cmd/safe-past
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 ./cmd/safe-past
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 ./cmd/safe-past
	GOOS=windows GOARCH=amd64 go build -ldflags "$(WIN_GUI_LDFLAGS)" -o dist/$(DAEMON)-windows-amd64.exe ./cmd/safe-past-daemon
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(DAEMON)-linux-amd64 ./cmd/safe-past-daemon

install: build
	cp bin/$(BINARY) /usr/local/bin/

clean:
	rm -rf bin dist
