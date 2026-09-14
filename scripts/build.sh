#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
exec "${GO:-go}" run "$root/scripts/release.go" -root "$root" -out "${1:-$HOME/Downloads/arbifx-http-release}"
