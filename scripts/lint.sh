#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/.."

version=v2.14.0
directory="$PWD/tools/golangci-lint/$version"
if [ ! -x "$directory/golangci-lint" ]; then
    installer=$(mktemp)
    trap 'rm -f "$installer"' EXIT
    curl -fsSL "https://raw.githubusercontent.com/golangci/golangci-lint/$version/install.sh" -o "$installer"
    sh "$installer" -b "$directory" "$version"
fi
"$directory/golangci-lint" run "$@"
