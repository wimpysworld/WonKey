# List available recipes
default:
    @just --list

# Build WonKey
build:
    go build -buildvcs=false -o wonkey ./cmd/wonkey

# Run Go tests
test:
    go test ./...

# Run linters
lint:
    @actionlint