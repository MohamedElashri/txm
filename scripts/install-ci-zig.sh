#!/usr/bin/env bash
set -euo pipefail
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source "$root/scripts/dependencies.env"
: "${RUNNER_TEMP:?CI runner temp required}" "${GITHUB_PATH:?GitHub PATH file required}"
dest=$(mktemp -d "$RUNNER_TEMP/txm-zig.XXXXXX")
case "$(uname -s):$(uname -m)" in
    Linux:x86_64) platform=x86_64-linux ;;
    Darwin:arm64) platform=aarch64-macos ;;
    Darwin:x86_64) platform=x86_64-macos ;;
    *) echo "Unsupported CI host" >&2; exit 1 ;;
esac
curl --fail --location --retry 3 https://ziglang.org/download/index.json -o "$dest/index.json"
url=$(jq -er --arg v "$ZIG_VERSION" --arg p "$platform" '.[$v][$p].tarball' "$dest/index.json")
checksum=$(jq -er --arg v "$ZIG_VERSION" --arg p "$platform" '.[$v][$p].shasum' "$dest/index.json")
curl --fail --location --retry 3 "$url" -o "$dest/zig.tar.xz"
if command -v sha256sum >/dev/null; then
    printf '%s  %s\n' "$checksum" "$dest/zig.tar.xz" | sha256sum -c
else
    printf '%s  %s\n' "$checksum" "$dest/zig.tar.xz" | shasum -a 256 -c
fi
tar -xf "$dest/zig.tar.xz" -C "$dest"
echo "$dest/zig-$platform-$ZIG_VERSION" >> "$GITHUB_PATH"
