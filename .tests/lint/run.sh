#!/usr/bin/env bash
set -euo pipefail

test_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$test_dir/../.." && pwd)"

cd -- "$repo_root"
golangci-lint run --config "$test_dir/.golangci.yml" ./...
