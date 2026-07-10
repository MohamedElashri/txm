.PHONY: build clean test lint init all build-deps

BINARY_NAME=txm
GHOSTTY_HOST_DIR ?= $(CURDIR)/.ghostty-host

all: test build

init:
	go mod download
	go mod tidy

build-deps:
	@if [ ! -d "ghostty" ]; then git clone https://github.com/ghostty-org/ghostty.git ghostty; fi
	@if [ ! -d "$(GHOSTTY_HOST_DIR)" ]; then cd ghostty && zig build -Demit-lib-vt --prefix $(GHOSTTY_HOST_DIR); fi

build: build-deps
	mkdir -p bin
	PKG_CONFIG_PATH=$(GHOSTTY_HOST_DIR)/share/pkgconfig CGO_ENABLED=1 go build -o bin/$(BINARY_NAME) .

clean:
	rm -rf bin
	rm -rf dist
	rm -rf $(GHOSTTY_HOST_DIR)

test: build-deps
	PKG_CONFIG_PATH=$(GHOSTTY_HOST_DIR)/share/pkgconfig CGO_ENABLED=1 go test -v ./...

lint: build-deps
	PKG_CONFIG_PATH=$(GHOSTTY_HOST_DIR)/share/pkgconfig CGO_ENABLED=1 golangci-lint run

install: build
	sudo ./bin/$(BINARY_NAME) install --system

install-user: build
	./bin/$(BINARY_NAME) install