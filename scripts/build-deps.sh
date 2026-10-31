#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source "$root/scripts/dependencies.env"
mode=${1:-host}
case "$mode" in host|release) ;; *) echo "usage: $0 [host|release]" >&2; exit 2 ;; esac
zig=${ZIG:-zig}
actual=$("$zig" version)
if [[ "$actual" != "$ZIG_VERSION" ]]; then
    echo "Zig $ZIG_VERSION required for Ghostty $GHOSTTY_REVISION; found $actual. Set ZIG to the verified executable." >&2
    exit 1
fi

src=${GHOSTTY_SOURCE_DIR:-$root/ghostty}
if [[ ! -e "$src" ]]; then
    # Never checkout/reset an existing source tree. An interrupted clone is
    # visibly incomplete and rejected on the next invocation.
    git init "$src"
    git -C "$src" remote add origin https://github.com/ghostty-org/ghostty.git
    git -C "$src" fetch --depth 1 origin "$GHOSTTY_REVISION"
    git -C "$src" checkout --detach FETCH_HEAD
fi
src=$(cd "$src" && pwd -P)
if [[ "$(git -C "$src" rev-parse --show-toplevel)" != "$src" ||
      "$(git -C "$src" rev-parse HEAD)" != "$GHOSTTY_REVISION" ||
      -n "$(git -C "$src" status --porcelain --untracked-files=normal)" ]]; then
    echo "Ghostty source must be a clean standalone checkout at $GHOSTTY_REVISION: $src (left untouched)" >&2
    exit 1
fi

build() {
    local prefix=$1 target=$2
    # txm links the static library, never an Apple XCFramework. Explicitly
    # disable packaging so Command Line Tools suffice without full Xcode.
    local args=(build -Demit-lib-vt -Demit-xcframework=false --prefix "$prefix")
    [[ "$target" == native ]] || args+=("-Dtarget=$target")
    [[ -z "${GHOSTTY_BUILD_JOBS:-}" ]] || args+=("-j$GHOSTTY_BUILD_JOBS")
    # Ask Zig to validate its content-based incremental cache every time.
    # Directory presence or our informational stamp is never a cache hit.
    (cd "$src" && "$zig" "${args[@]}")
    test -s "$prefix/share/pkgconfig/libghostty-vt-static.pc"
    test -s "$prefix/lib/libghostty-vt.a"
    printf 'ghostty=%s\nzig=%s\ntarget=%s\n' "$GHOSTTY_REVISION" "$actual" "$target" > "$prefix/.txm-build-info"
}

if [[ "$mode" == host ]]; then
    prefix=${GHOSTTY_HOST_DIR:-$root/.ghostty-host}
    mkdir -p "$prefix"
    prefix=$(cd "$prefix" && pwd -P)
    build "$prefix" native
else
    if [[ "${TXM_RELEASE_ROOT:-}" != "$root" ]]; then
        echo "Set TXM_RELEASE_ROOT to the absolute repository root: $root" >&2
        exit 1
    fi
    # These absolute paths match .goreleaser.yaml; pkg-config runs in module dirs.
    for tuple in x86_64-linux-gnu:linux-amd64 aarch64-linux-gnu:linux-arm64 \
        x86_64-linux-musl:linux-musl-amd64 aarch64-linux-musl:linux-musl-arm64 \
        x86_64-macos:darwin-amd64 aarch64-macos:darwin-arm64; do
        build "$root/.ghostty-release/${tuple#*:}" "${tuple%%:*}"
    done
fi
