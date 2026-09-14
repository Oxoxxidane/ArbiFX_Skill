#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
case "$(uname -s)" in Darwin) os=darwin;; Linux) os=linux;; *) echo 'Unsupported OS' >&2; exit 2;; esac
case "$(uname -m)" in x86_64|amd64) arch=amd64;; arm64|aarch64) arch=arm64;; *) echo 'Unsupported CPU' >&2; exit 2;; esac
dest=${1:-"$HOME/.local/bin"}
mkdir -p "$dest"
existing=$(command -v arbifx || true)
if [ -n "$existing" ] && [ "$existing" != "$dest/arbifx" ]; then
    echo "arbifx command already exists: $existing" >&2; exit 2
fi
cp "$root/bin/$os-$arch/arbifx" "$dest/arbifx"
chmod 755 "$dest/arbifx"
"$dest/arbifx" --version
printf 'Installed: %s/arbifx\nAdd this directory to PATH if needed.\n' "$dest"
