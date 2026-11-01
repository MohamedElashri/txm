#!/usr/bin/env python3
"""Verify the complete release bundle before any GitHub draft upload."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import shutil
import struct
import zipfile

REPO = Path(__file__).resolve().parents[1]
COMMON = {'LICENSE', 'README.md', 'docs/txm.1', 'completions/txm.bash',
          'completions/_txm', 'completions/txm.fish', 'completions/txm.ps1'}
TARGETS = {f'txm_{system}_{arch}{suffix}.zip': (system, arch, suffix == '-musl')
           for system, suffixes in [('Linux', ['', '-musl']), ('Windows', [''])]
           for arch in ['x86_64', 'arm64'] for suffix in suffixes}
TARGETS['txm_Darwin_universal.zip'] = ('Darwin', 'universal', False)


def universal_darwin(data):
    """Check both bounded, nonoverlapping Mach-O slices, not just the fat header."""
    magic, count = struct.unpack_from('>II', data)
    if magic not in (0xcafebabe, 0xcafebabf) or count != 2:
        raise ValueError('Darwin universal binary must contain exactly two slices')
    wide = magic == 0xcafebabf
    entry_size = 32 if wide else 20
    header_end = 8 + count * entry_size
    targets = {0x1000007: 'x86_64', 0x100000c: 'arm64'}
    seen, ranges = set(), []
    for index in range(count):
        entry = struct.unpack_from('>IIQQII' if wide else '>IIIII', data, 8 + index * entry_size)
        cpu, subtype, offset, size, alignment = entry[:5]
        if (cpu not in targets or cpu in seen or alignment > 30 or offset < header_end
                or offset % (1 << alignment) or size < 32 or offset + size > len(data)
                or wide and entry[5] != 0):
            raise ValueError('invalid Darwin universal slice')
        if any(offset < end and start < offset + size for start, end in ranges):
            raise ValueError('overlapping Darwin universal slices')
        slice_data = data[offset:offset + size]
        binary_target(slice_data, 'Darwin', targets[cpu])
        if struct.unpack_from('<I', slice_data, 8)[0] != subtype:
            raise ValueError('Darwin universal CPU subtype mismatch')
        seen.add(cpu)
        ranges.append((offset, offset + size))


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
            if arch == 'universal':
                universal_darwin(data)
            elif data[:4] != b'\xcf\xfa\xed\xfe' or struct.unpack_from('<I', data, 4)[0] != {'x86_64': 0x1000007, 'arm64': 0x100000c}[arch]:
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


def verify(directory, repo=REPO, platform=None):
    targets = {name: target for name, target in TARGETS.items()
               if platform is None or (target[0] == 'Darwin') == (platform == 'darwin')}
    manifest_name = 'checksums.txt' if platform is None else f'checksums-{platform}.txt'
    expected = {*targets, manifest_name}
    entries = list(directory.iterdir())
    if {p.name for p in entries} != expected or any(p.is_symlink() or not p.is_file() for p in entries):
        raise ValueError(f'bundle must contain exactly {len(targets)} regular ZIP archives and {manifest_name}')
    manifest = {}
    for line in (directory/manifest_name).read_text().splitlines():
        match = re.fullmatch(r'([a-fA-F0-9]{64})\s+\*?([^\s/\\]+)', line)
        if not match or match[2] not in targets or match[2] in manifest:
            raise ValueError('invalid, duplicate or unexpected checksum entry')
        manifest[match[2]] = match[1].lower()
    if set(manifest) != set(targets):
        raise ValueError('checksums must cover every expected archive')
    result = []
    for name, (system, arch, static) in targets.items():
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


def assemble(parts, directory, repo=REPO):
    """Merge the two build partitions without regenerating trusted checksums."""
    manifests = {'checksums-linux.txt', 'checksums-darwin.txt'}
    entries = list(parts.iterdir())
    if ({p.name for p in entries} != set(TARGETS) | manifests
            or any(p.is_symlink() or not p.is_file() for p in entries)):
        raise ValueError(f'build partitions must contain exactly {len(TARGETS)} ZIPs and two checksum manifests')
    checksums = []
    for platform in ('linux', 'darwin'):
        lines = (parts/f'checksums-{platform}.txt').read_text().splitlines()
        expected = {name for name, (system, _, _) in TARGETS.items()
                    if (system == 'Darwin') == (platform == 'darwin')}
        names = []
        for line in lines:
            match = re.fullmatch(r'[a-fA-F0-9]{64}\s+\*?([^\s/\\]+)', line)
            if not match:
                raise ValueError('invalid partition checksum entry')
            names.append(match[1])
        if len(names) != len(expected) or set(names) != expected:
            raise ValueError('partition checksum inventory mismatch: '+platform)
        checksums.extend(lines)
    directory.mkdir()  # Refuse an existing bundle, including a previous attempt.
    for name in TARGETS:
        shutil.copyfile(parts/name, directory/name)
    (directory/'checksums.txt').write_text('\n'.join(sorted(checksums))+'\n')
    return verify(directory, repo)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--directory', required=True, type=Path)
    parser.add_argument('--assemble-from', type=Path, help='Assemble and verify Linux and Darwin build partitions')
    parser.add_argument('--partition', choices=['linux', 'darwin'], help='Verify only one build partition before assembly')
    parser.add_argument('--report', type=Path)
    args = parser.parse_args()
    if args.partition and args.assemble_from:
        parser.error('--partition and --assemble-from are mutually exclusive')
    try:
        result = assemble(args.assemble_from, args.directory) if args.assemble_from else verify(args.directory, platform=args.partition)
        if args.report:
            with args.report.open('x') as out:
                out.write(json.dumps(result, indent=2)+'\n')
    except (OSError, ValueError, zipfile.BadZipFile) as error:
        parser.exit(1, str(error)+'\n')
    print(json.dumps(result))


if __name__ == '__main__':
    main()
