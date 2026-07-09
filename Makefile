.PHONY: build clean test lint init all build-deps

BINARY_NAME=txm

all: test build

init:
	go mod download
	go mod tidy

build-deps:
	@if [ ! -d "ghostty" ]; then git clone https://github.com/ghostty-org/ghostty.git ghostty; fi
	cd ghostty && zig build -Demit-lib-vt --prefix /tmp/ghostty-host

build: build-deps
	mkdir -p bin
	PKG_CONFIG_PATH=/tmp/ghostty-host/share/pkgconfig CGO_ENABLED=1 go build -o bin/$(BINARY_NAME) .

clean:
	rm -rf bin
	rm -rf dist

test: build-deps
	PKG_CONFIG_PATH=/tmp/ghostty-host/share/pkgconfig CGO_ENABLED=1 go test -v ./...

lint: build-deps
	PKG_CONFIG_PATH=/tmp/ghostty-host/share/pkgconfig CGO_ENABLED=1 golangci-lint run

install: build
	sudo ./bin/$(BINARY_NAME) install --system

install-user: build
	./bin/$(BINARY_NAME) install