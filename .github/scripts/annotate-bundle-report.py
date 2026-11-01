#!/usr/bin/env python3
"""Bind archive validation and a single privacy scan to bounded CI evidence."""
import argparse
import hashlib
import json
from pathlib import Path
import re


def proof(bundle, privacy_raw, commit):
    privacy = json.loads(privacy_raw)
    if not re.fullmatch(r'[0-9a-f]{40}', commit):
        raise ValueError('expected exact source commit')
    if bundle.get('passed') is not True or privacy.get('passed') is not True or privacy.get('findings') != []:
        raise ValueError('bundle validation and privacy checks must pass')
    expected = {row['archive']: row['sha256'] for row in bundle['archives']}
    scanned = {Path(row['path']).name: row['sha256'] for row in privacy['artifacts']}
    if (len(bundle['archives']) != 7 or len(expected) != 7 or len(privacy['artifacts']) != 7
            or expected != scanned or any(not re.fullmatch(r'[0-9a-f]{64}', value) for value in expected.values())):
        raise ValueError('privacy scan must match all seven verified archives exactly')
    result = {'source_sha': commit, 'archives': [
        {'archive': name, 'sha256': digest} for name, digest in sorted(expected.items())],
        'privacy': {'passed': True, 'findings': 0, 'archives_scanned': 7,
                    'report_sha256': hashlib.sha256(privacy_raw).hexdigest()}}
    text = json.dumps(result, separators=(',', ':'))
    if len(text.encode()) >= 3000:
        raise ValueError('bundle proof exceeds annotation budget')
    return text


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--bundle-check', required=True, type=Path)
    parser.add_argument('--privacy', required=True, type=Path)
    parser.add_argument('--commit', required=True)
    args = parser.parse_args()
    text = proof(json.loads(args.bundle_check.read_text()), args.privacy.read_bytes(), args.commit)
    escaped = text.replace('%', '%25').replace('\r', '%0D').replace('\n', '%0A')
    print('::notice title=Verified bundle proof::'+escaped)


if __name__ == '__main__':
    main()
