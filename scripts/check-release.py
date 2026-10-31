#!/usr/bin/env python3
"""Check release metadata and optionally write notes from the canonical changelog."""
import argparse
import json
from pathlib import Path
import re

REPO = Path(__file__).resolve().parents[1]
SEMVER = re.compile(r'(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?')


def release_notes(repo, tag=None, stable=False):
    version = (repo/'VERSION').read_text().strip()
    match = SEMVER.fullmatch(version)
    if not match or any(x.isdigit() and len(x) > 1 and x[0] == '0'
                        for x in (match[4] or '').split('.')):
        raise ValueError('VERSION must contain a complete semantic release version')
    if stable and match[4] is not None:
        raise ValueError('stable release must not have a prerelease suffix')
    if tag is not None and tag != 'v'+version:
        raise ValueError('release tag must match v'+version)
    text = (repo/'docs/CHANGELOG.md').read_text()
    pattern = r'^## \['+re.escape(version)+r'\] - (\d{4}-\d{2}-\d{2})\n(.*?)(?=^## \[|\Z)'
    sections = list(re.finditer(pattern, text, re.M | re.S))
    if len(sections) != 1 or not sections[0][2].strip():
        raise ValueError('changelog must contain exactly one dated, nonempty VERSION section')
    if not re.search(r'^### Upgrade notes$', sections[0][2], re.M):
        raise ValueError('release notes must include upgrade notes')
    notes = '# txm '+version+'\n\n'+sections[0][2].strip()+'\n'
    # Notes are published at repository release URLs, not from the docs directory.
    notes = re.sub(r'\]\(([^:/#)]+\.md)(#[^)]*)?\)',
                   lambda m: ']('+f'https://github.com/MohamedElashri/txm/blob/v{version}/docs/'+m[1]+(m[2] or '')+')', notes)
    return version, notes


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--tag', help='Require this exact v-prefixed tag to match VERSION')
    parser.add_argument('--stable', action='store_true', help='Reject prerelease versions')
    parser.add_argument('--notes', type=Path, help='Write release notes to a new output file')
    args = parser.parse_args()
    try:
        version, notes = release_notes(REPO, args.tag, args.stable)
        if args.notes:
            with args.notes.open('x') as output:
                output.write(notes)
    except (OSError, ValueError) as error:
        parser.exit(1, str(error)+'\n')
    print(json.dumps({'version': version, 'tag': 'v'+version, 'metadata': 'pass'}))


if __name__ == '__main__':
    main()
