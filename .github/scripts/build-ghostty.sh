#!/bin/bash
set -e

echo "Cloning ghostty..."
if [ ! -d "ghostty" ]; then
    git clone https://github.com/ghostty-org/ghostty.git ghostty
fi

cd ghostty

targets=(
    "x86_64-linux-gnu:linux:amd64"
    "aarch64-linux-gnu:linux:arm64"
    "x86_64-linux-musl:linux-musl:amd64"
    "aarch64-linux-musl:linux-musl:arm64"
    "x86_64-macos:darwin:amd64"
    "aarch64-macos:darwin:arm64"
    "x86_64-windows-gnu:windows:amd64"
    "aarch64-windows-gnu:windows:arm64"
)

for tgt in "${targets[@]}"; do
    IFS=':' read -r zig_target goos goarch <<< "$tgt"
    echo "Building libghostty-vt for $zig_target ($goos/$goarch)..."
    zig build -Demit-lib-vt -Dtarget=$zig_target --prefix /tmp/ghostty-$goos-$goarch
done
