import 'just/loader.just'

# List available recipes
default:
    @just --list

# Build WonKey
build-wonkey:
    go build -buildvcs=false -o wonkey ./cmd/wonkey

# Preview static pages
pages-port PORT="18080":
    miniserve --index index.html --interfaces 127.0.0.1 --port {{quote(PORT)}} pages/

# Create a local release tag (usage: just release-wonkey x.y.z)
release-wonkey VERSION:
    @just release {{quote(VERSION)}}
    @echo "This will trigger GoReleaser via GitHub Actions which will:"
    @echo "  - Cross-compile binaries for Linux (amd64, arm64)"
    @echo "  - Generate changelog from commits"
    @echo "  - Create GitHub release with tarballs and checksums"
    @echo "  - Build and publish native packages (deb, rpm, apk)"
    @echo "  - Publish the Nix package to wimpysworld/nix-packages"
    @echo "  - Publish xfkey-wonkey-bin to the AUR"
