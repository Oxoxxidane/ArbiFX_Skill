#!/bin/sh
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
case "$(uname -s)" in Darwin) os=darwin;; Linux) os=linux;; *) echo 'Unsupported OS' >&2; exit 2;; esac
case "$(uname -m)" in x86_64|amd64) arch=amd64;; arm64|aarch64) arch=arm64;; *) echo 'Unsupported CPU' >&2; exit 2;; esac
binary="$root/bin/$os-$arch/arbifx"
if [ ! -x "$binary" ]; then chmod u+x "$binary"; fi
exec "$binary" "$@"
