#!/usr/bin/env python3
"""Verify the complete release bundle before any GitHub draft upload."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import struct
import zipfile

REPO = Path(__file__).resolve().parents[1]
COMMON = {'LICENSE', 'README.md', 'docs/txm.1', 'completions/txm.bash',
          'completions/_txm', 'completions/txm.fish', 'completions/txm.ps1'}
TARGETS = {f'txm_{system}_{arch}{suffix}.zip': (system, arch, suffix == '-musl')
           for system, suffixes in [('Linux', ['', '-musl']), ('Darwin', ['']), ('Windows', [''])]
           for arch in ['x86_64', 'arm64'] for suffix in suffixes}


def binary_target(data, system, arch, static=False):
    try:
        if system == 'Linux':
            if data[:6] != b'\x7fELF\x02\x01' or struct.unpack_from('<H', data, 18)[0] != {'x86_64': 62, 'arm64': 183}[arch]:
                raise ValueError('wrong ELF target')
            if static:
                offset = struct.unpack_from('<Q', data, 32)[0]
                size, count = struct.unpack_from('<HH', data, 54)
                if not count or size < 56 or offset+size*count > len(data):
                    raise ValueError('invalid ELF program headers')
                if any(struct.unpack_from('<I', data, offset+i*size)[0] in (2, 3) for i in range(count)):
                    raise ValueError('musl executable must be static, without interpreter/dynamic headers')
        elif system == 'Darwin':
            if data[:4] != b'\xcf\xfa\xed\xfe' or struct.unpack_from('<I', data, 4)[0] != {'x86_64': 0x1000007, 'arm64': 0x100000c}[arch]:
                raise ValueError('wrong Mach-O target')
        elif system == 'Windows':
            if data[:2] != b'MZ':
                raise ValueError('wrong PE signature')
            offset = struct.unpack_from('<I', data, 60)[0]
            if data[offset:offset+4] != b'PE\0\0' or struct.unpack_from('<H', data, offset+4)[0] != {'x86_64': 0x8664, 'arm64': 0xaa64}[arch]:
                raise ValueError('wrong PE target')
        else:
            raise ValueError('unknown binary target')
    except struct.error as e:
        raise ValueError('truncated executable header') from e


def verify(directory, repo=REPO):
    expected = {*TARGETS, 'checksums.txt'}
    entries = list(directory.iterdir())
    if {p.name for p in entries} != expected or any(p.is_symlink() or not p.is_file() for p in entries):
        raise ValueError('bundle must contain exactly eight regular ZIP archives and checksums.txt')
    manifest = {}
    for line in (directory/'checksums.txt').read_text().splitlines():
        match = re.fullmatch(r'([a-fA-F0-9]{64})\s+\*?([^\s/\\]+)', line)
        if not match or match[2] not in TARGETS or match[2] in manifest:
            raise ValueError('invalid, duplicate or unexpected checksum entry')
        manifest[match[2]] = match[1].lower()
    if set(manifest) != set(TARGETS):
        raise ValueError('checksums must cover all eight archives')
    result = []
    for name, (system, arch, static) in TARGETS.items():
        path = directory/name
        if path.stat().st_size > 200 << 20:
            raise ValueError('oversized ZIP archive')
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        if digest != manifest[name]:
            raise ValueError('checksum mismatch: '+name)
        executable = 'txm.exe' if system == 'Windows' else 'txm'
        with zipfile.ZipFile(path) as payload:
            members = payload.infolist()
            if len(members) != len(COMMON)+1 or {i.filename for i in members} != COMMON|{executable}:
                raise ValueError('unexpected archive inventory: '+name)
            if any(not 0 < i.file_size <= 100 << 20 or (i.external_attr >> 16) & 0o170000 == 0o120000 for i in members):
                raise ValueError('invalid ZIP member: '+name)
            for source in ['README.md', 'LICENSE', 'docs/txm.1']:
                if payload.read(source) != (repo/source).read_bytes():
                    raise ValueError('packaged source mismatch: '+name+'/'+source)
            binary_target(payload.read(executable), system, arch, static)
        result.append({'archive': name, 'sha256': digest, 'os': system, 'arch': arch, 'static_musl': static})
    return {'passed': True, 'archives': result, 'scope': 'Inventory, source bytes, checksums and binary targets; host runtime checks are separate'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--directory', required=True, type=Path)
    parser.add_argument('--report', type=Path)
    args = parser.parse_args()
    try:
        result = verify(args.directory)
        if args.report:
            with args.report.open('x') as out:
                out.write(json.dumps(result, indent=2)+'\n')
    except (OSError, ValueError, zipfile.BadZipFile) as error:
        parser.exit(1, str(error)+'\n')
    print(json.dumps(result))


if __name__ == '__main__':
    main()
