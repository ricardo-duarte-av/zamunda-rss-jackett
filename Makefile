.PHONY: build run clean deps test

# Build the application
build:
	go build -o bin/zamunda-rss-jackett main.go

# Run the application
run:
	go run main.go

# Install dependencies
deps:
	go mod tidy
	go mod download

# Clean build artifacts
clean:
	rm -rf bin/

# Run tests (if any)
test:
	go test ./...

# Build for different platforms
build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/zamunda-rss-jackett-linux main.go

build-windows:
	GOOS=windows GOARCH=amd64 go build -o bin/zamunda-rss-jackett.exe main.go

build-mac:
	GOOS=darwin GOARCH=amd64 go build -o bin/zamunda-rss-jackett-mac main.go

# Build all platforms
build-all: build-linux build-windows build-mac

# Create directories
setup:
	mkdir -p bin
