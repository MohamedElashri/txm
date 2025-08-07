.PHONY: build clean install test init check-deps all

all: check-deps build

# Check if go.mod and go.sum exist and are properly initialized
check-deps:
	@echo "Checking module initialization..."
	@if [ ! -f go.mod ]; then \
		echo "Error: go.mod not found. Please run 'make init' first."; \
		exit 1; \
	fi
	@if [ ! -f go.sum ]; then \
		echo "Warning: go.sum not found. Running go mod tidy..."; \
		go mod tidy; \
	fi
	@echo "Verifying dependencies..."
	@go mod verify || (echo "Dependencies verification failed. Running go mod tidy..." && go mod tidy)

# Initialize or update module dependencies
init:
	@echo "Initializing module..."
	@if [ ! -f go.mod ]; then \
		go mod init txm; \
	fi
	@go mod tidy
	@echo "Module initialized successfully"

build: check-deps
	@echo "Building txm..."
	@mkdir -p bin
	@go build -o bin/txm .

clean:
	@echo "Cleaning..."
	@rm -rf bin
	@rm -f go.sum

install: build
	@echo "Installing to /usr/local/bin..."
	@sudo cp bin/txm /usr/local/bin/

install-user: build
	@echo "Installing to ~/.local/bin..."
	@mkdir -p ~/.local/bin
	@cp bin/txm ~/.local/bin/

test: check-deps
	@echo "Running tests..."
	@go test ./...