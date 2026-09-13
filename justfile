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
    @golangci-lint run ./...
    @govulncheck ./...

# Create a local release tag (usage: just release x.y.z)
release VERSION:
    #!/usr/bin/env bash
    set -e

    VERSION={{quote(VERSION)}}
    if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "Error: VERSION must be in format x.y.z (for example, 0.1.0)"
        exit 1
    fi

    if [ -n "$(git status --porcelain)" ]; then
        echo "Error: Working directory is not clean"
        exit 1
    fi

    if git show-ref --tags --verify --quiet "refs/tags/v$VERSION"; then
        echo "Error: Tag v$VERSION already exists"
        exit 1
    fi

    echo "Creating release v$VERSION..."
    git tag -a "v$VERSION" -m "v$VERSION"
    echo "Tag v$VERSION created"
    echo ""
    echo "To publish the release:"
    echo "  git push origin v$VERSION"
    echo ""
    echo "This will trigger GoReleaser via GitHub Actions which will:"
    echo "  - Cross-compile binaries for Linux (amd64, arm64)"
    echo "  - Generate changelog from commits"
    echo "  - Create GitHub release with tarballs and checksums"
    echo "  - Build and publish native packages (deb, rpm, apk)"
    echo "  - Publish the Nix package to wimpysworld/nix-packages"
    echo "  - Publish wonkey-bin to the AUR"
