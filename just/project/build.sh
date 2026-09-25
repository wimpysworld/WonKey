#!/usr/bin/env bash
set -euo pipefail

go build -buildvcs=false -o wonkey ./cmd/wonkey
