#!/usr/bin/env python3
"""Scope Go's cache to the verified external static libraries it will link."""
import argparse
import hashlib
from pathlib import Path


def fingerprint(root, mode, platform, seed):
    if mode == 'host':
        prefixes = ['.ghostty-host']
    elif platform == 'linux':
        prefixes = ['.ghostty-release/'+target for target in
                    ['linux-amd64', 'linux-arm64', 'linux-musl-amd64', 'linux-musl-arm64']]
    elif platform == 'darwin':
        prefixes = ['.ghostty-release/darwin-amd64', '.ghostty-release/darwin-arm64']
    else:
        raise ValueError('release cache requires a Linux or Darwin partition')
    digest = hashlib.sha256(seed.encode())
    for prefix in prefixes:
        relative = prefix+'/lib/libghostty-vt.a'
        path = root/relative
        if path.is_symlink() or not path.is_file() or not path.stat().st_size:
            raise ValueError('missing regular native library: '+relative)
        digest.update(b'\0'+relative.encode()+b'\0')
        library = hashlib.sha256()
        with path.open('rb') as source:
            for block in iter(lambda: source.read(1024*1024), b''):
                library.update(block)
        digest.update(library.digest())
    return digest.hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root', type=Path, default=Path.cwd())
    parser.add_argument('--mode', choices=['host', 'release'], required=True)
    parser.add_argument('--platform', choices=['linux', 'darwin'])
    parser.add_argument('--seed', required=True)
    args = parser.parse_args()
    try:
        print(fingerprint(args.root, args.mode, args.platform, args.seed))
    except (OSError, ValueError) as error:
        parser.exit(1, str(error)+'\n')


if __name__ == '__main__':main()
