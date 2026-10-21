BINARY=safe-past
VERSION?=dev
LDFLAGS=-s -w -X main.version=$(VERSION)

.PHONY: test build build-all install clean

test:
	go test ./...

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/safe-past

build-all:
	GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-windows-amd64.exe ./cmd/safe-past
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-darwin-arm64 ./cmd/safe-past
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-linux-amd64 ./cmd/safe-past

install: build
	cp bin/$(BINARY) /usr/local/bin/

clean:
	rm -rf bin dist
