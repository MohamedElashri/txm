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
# Hosted macOS checksum shims differ from GNU utilities. Use the same
# streaming SHA-256 implementation on every CI host, and fail before extraction.
python3 - "$checksum" "$dest/zig.tar.xz" <<'PY_CHECKSUM'
import hashlib
import re
import sys

expected, path = sys.argv[1:]
if not re.fullmatch(r"[0-9a-fA-F]{64}", expected):
    raise SystemExit("Invalid Zig archive checksum metadata")
digest = hashlib.sha256()
with open(path, "rb") as archive:
    for block in iter(lambda: archive.read(1024 * 1024), b""):
        digest.update(block)
if digest.hexdigest() != expected.lower():
    raise SystemExit("Zig archive SHA-256 mismatch")
print("Zig archive SHA-256 verified")
PY_CHECKSUM
tar -xf "$dest/zig.tar.xz" -C "$dest"
echo "$dest/zig-$platform-$ZIG_VERSION" >> "$GITHUB_PATH"
