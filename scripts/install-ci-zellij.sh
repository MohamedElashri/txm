#!/usr/bin/env bash
# Test the documented Zellij version, independent of runner Homebrew snapshots.
set -euo pipefail
: "${RUNNER_TEMP:?CI runner temp required}" "${GITHUB_PATH:?GitHub PATH file required}"
version=0.45.1
case "$(uname -s):$(uname -m)" in
    Darwin:arm64)
        target=aarch64-apple-darwin
        checksum=c029ba4fe1927b79ad9f0cdd59155c4dff80777863c85857d4d09b88b56f9891 ;;
    Darwin:x86_64)
        target=x86_64-apple-darwin
        checksum=8e8bea22737d1652278c51fc5c26c7c22c9855d0ebb9634a84b8873823093114 ;;
    *) echo "Unsupported Zellij CI host" >&2; exit 1 ;;
esac
dest=$(mktemp -d "$RUNNER_TEMP/txm-zellij.XXXXXX")
curl --fail --location --retry 3 \
    "https://github.com/zellij-org/zellij/releases/download/v$version/zellij-$target.tar.gz" \
    -o "$dest/zellij.tar.gz"
python3 - "$checksum" "$dest/zellij.tar.gz" <<'PY_CHECKSUM'
import hashlib
import sys

expected, path = sys.argv[1:]
with open(path, "rb") as archive:
    actual = hashlib.file_digest(archive, "sha256").hexdigest()
if actual != expected:
    raise SystemExit("Zellij archive SHA-256 mismatch")
print("Zellij archive SHA-256 verified")
PY_CHECKSUM
tar -xzf "$dest/zellij.tar.gz" -C "$dest" zellij
chmod +x "$dest/zellij"
test "$("$dest/zellij" --version)" = "zellij $version"
echo "$dest" >> "$GITHUB_PATH"
