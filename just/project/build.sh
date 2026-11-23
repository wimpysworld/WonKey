#!/usr/bin/env bash
set -euo pipefail

VERSION=$(git describe --tags --match 'v*.*.*' --always --dirty 2>/dev/null || echo "dev")
VERSION="${VERSION#v}"
go build -buildvcs=false -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o wonkey ./cmd/wonkey
