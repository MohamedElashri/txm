#!/usr/bin/env python3
"""Scan explicit artifacts for known credential forms without printing matches.

This is a bounded pattern check, not a proof that arbitrary prose contains no
secrets. Pair it with synthetic fixture provenance and human source review.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import zipfile

RULES = {
    'private-key': re.compile(rb'-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----[\r\n]+[A-Za-z0-9+/=\r\n]{64,16384}-----END (?:RSA |EC |OPENSSH )?PRIVATE KEY-----'),
    'aws-access-key': re.compile(rb'(?<![A-Za-z0-9])(?:AKIA|ASIA)[A-Z0-9]{16}(?![A-Za-z0-9])'),
    'github-token': re.compile(rb'gh[pousr]_[A-Za-z0-9]{36,255}(?![A-Za-z0-9])'),
    'openai-token': re.compile(rb'sk-(?:proj-|svcacct-)[A-Za-z0-9_-]{48,255}'),
}


def scan_stream(stream):
    seen = set()
    tail = b''
    offset = 0
    while True:
        block = stream.read(1 << 20)
        if not block:
            break
        content = tail+block
        for name, pattern in RULES.items():
            for match in pattern.finditer(content):
                seen.add((name, offset-len(tail)+match.start()))
        offset += len(block)
        tail = content[-32768:]
    return [{'rule': rule, 'offset': pos} for rule, pos in sorted(seen)]


def audit(paths):
    findings, artifacts = [], []
    for path in sorted(set(paths)):
        digest = hashlib.sha256()
        with path.open('rb') as stream:
            for chunk in iter(lambda: stream.read(1 << 20), b''):
                digest.update(chunk)
        artifacts.append({'path': str(path), 'sha256': digest.hexdigest(), 'bytes': path.stat().st_size})
        with path.open('rb') as stream:
            for finding in scan_stream(stream):
                findings.append({'path': str(path), **finding})
        if zipfile.is_zipfile(path):
            with zipfile.ZipFile(path) as payload:
                members = payload.infolist()
                if len(members) > 128 or sum(i.file_size for i in members) > 512 << 20:
                    findings.append({'path': str(path), 'rule': 'oversized-archive'})
                    continue
                for item in members:
                    if item.file_size > 100 << 20:
                        findings.append({'path': str(path), 'member': item.filename, 'rule': 'oversized-member'})
                        continue
                    with payload.open(item) as stream:
                        for finding in scan_stream(stream):
                            findings.append({'path': str(path), 'member': item.filename, **finding})
    return {'schema_version': 1, 'scope': 'Known credential forms only; no match contents logged; human provenance review required',
            'artifacts': artifacts, 'findings': findings, 'passed': not findings}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('paths', nargs='+', type=Path)
    parser.add_argument('--report', required=True, type=Path)
    args = parser.parse_args()
    result = audit([p.resolve(strict=True) for p in args.paths])
    with args.report.open('x') as output:
        json.dump(result, output, indent=2)
        output.write('\n')
    print(json.dumps({'passed': result['passed'], 'findings': len(result['findings']), 'report': str(args.report)}))
    return 0 if result['passed'] else 1


if __name__ == '__main__':
    raise SystemExit(main())
