.PHONY: build clean test lint init update all build-deps test-integration test-race test-harness test-fuzz test-integration-race test-performance test-extensions test-ssh release-check

BINARY_NAME=txm
VERSION ?= $(shell cat VERSION)
VERSION_LDFLAGS = -X github.com/MohamedElashri/txm/pkg/cmd.Version=$(VERSION)
GHOSTTY_HOST_DIR ?= $(CURDIR)/.ghostty-host
GHOSTTY_SOURCE_DIR ?= $(CURDIR)/ghostty
ZIG ?= zig

all: test build

init:
	go mod download
	go mod tidy

update:
	go get -u ./...
	go mod tidy

build-deps:
	GHOSTTY_SOURCE_DIR="$(GHOSTTY_SOURCE_DIR)" GHOSTTY_HOST_DIR="$(GHOSTTY_HOST_DIR)" ZIG="$(ZIG)" bash scripts/build-deps.sh host

build: build-deps
	mkdir -p bin
	PKG_CONFIG_PATH="$(GHOSTTY_HOST_DIR)/share/pkgconfig" CGO_ENABLED=1 go build -ldflags "$(VERSION_LDFLAGS)" -o bin/$(BINARY_NAME) .

clean:
	rm -rf bin
	rm -rf dist
	rm -rf $(GHOSTTY_HOST_DIR)

test: build-deps
	PKG_CONFIG_PATH="$(GHOSTTY_HOST_DIR)/share/pkgconfig" CGO_ENABLED=1 go test -v ./...

lint: build-deps
	PKG_CONFIG_PATH="$(GHOSTTY_HOST_DIR)/share/pkgconfig" CGO_ENABLED=1 golangci-lint run

test-harness:
	python3 -m unittest discover -s tests -p 'test_*.py' -v

test-integration: build test-harness
	python3 tests/cli_errors.py --binary "$(CURDIR)/bin/$(BINARY_NAME)"
	python3 tests/native_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)"
	python3 tests/structured_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)"
	python3 tests/profile_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)"
	python3 tests/agent_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)"
	python3 tests/agent_events_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)"
	python3 tests/dashboard_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)"
	python3 tests/extension_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)"
	python3 tests/prune_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)"

test-extensions: build test-harness
	python3 tests/extension_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)"
	python3 tests/prune_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)"

test-race: build-deps
	PKG_CONFIG_PATH="$(GHOSTTY_HOST_DIR)/share/pkgconfig" CGO_ENABLED=1 go test -race ./...

test-fuzz:
	go test ./pkg/nativeprotocol -run='^$$' -fuzz=FuzzNativeFrame -fuzztime=30s -parallel=2
	go test ./pkg/core -run='^$$' -fuzz=FuzzPlain -fuzztime=10s -parallel=2
	go test ./pkg/localipc -run='^$$' -fuzz=FuzzLocalFrame -fuzztime=15s -parallel=2
	go test ./pkg/remote -run='^$$' -fuzz=FuzzDecode -fuzztime=15s -parallel=2

test-integration-race: build-deps test-harness
	mkdir -p bin
	PKG_CONFIG_PATH="$(GHOSTTY_HOST_DIR)/share/pkgconfig" CGO_ENABLED=1 go build -race -ldflags "$(VERSION_LDFLAGS)" -o bin/$(BINARY_NAME)-race .
	python3 tests/native_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)-race"
	python3 tests/structured_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)-race"
	python3 tests/profile_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)-race"
	python3 tests/agent_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)-race"
	python3 tests/agent_events_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)-race"
	python3 tests/dashboard_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)-race"
	python3 tests/extension_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)-race"
	python3 tests/prune_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)-race"

test-performance: build test-harness
	python3 tests/performance.py --binary "$(CURDIR)/bin/$(BINARY_NAME)" --report "$(CURDIR)/bin/performance.json"

install: build
	sudo ./bin/$(BINARY_NAME) install --system

install-user: build
	./bin/$(BINARY_NAME) install

# Real loopback OpenSSH; unprivileged macOS or the isolated Linux container.
test-ssh: build test-harness
	python3 tests/ssh_checks.py --binary "$(CURDIR)/bin/$(BINARY_NAME)" --report "$(CURDIR)/bin/ssh.json" -v

release-check:
	python3 scripts/check-release.py --stable
	goreleaser check
