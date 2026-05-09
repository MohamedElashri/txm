.PHONY: build clean test lint init all

BINARY_NAME=txm

all: test build

init:
	go mod download
	go mod tidy

build:
	mkdir -p bin
	go build -o bin/$(BINARY_NAME) .

clean:
	rm -rf bin
	rm -rf dist

test:
	go test -v ./...

lint:
	golangci-lint run

install: build
	sudo ./bin/$(BINARY_NAME) install --system

install-user: build
	./bin/$(BINARY_NAME) install