#!/usr/bin/env python3
"""Expose bounded, hash-bound package proof through the Actions annotations API."""
import argparse
import hashlib
import json
from pathlib import Path
import re


def summaries(directory, archive, version, commit, system):
    if not re.fullmatch(r'[0-9a-f]{40}', commit):
        raise ValueError('expected exact source commit')
    windows = system == 'Windows'
    wanted = {'windows-cli.json'} if windows else {'install.json', 'peer-credentials.json'}
    if {p.name for p in directory.iterdir()} != wanted:
        raise ValueError('missing or unexpected package report')
    with archive.open('rb') as stream:
        digest = hashlib.file_digest(stream, 'sha256').hexdigest()
    rows = []
    for name in sorted(wanted):
        raw = (directory/name).read_bytes()
        if len(raw) > 65536:
            raise ValueError('oversized package report')
        report = json.loads(raw)
        if report.get('passed') is not True:
            raise ValueError('package check did not pass')
        if name != 'peer-credentials.json':
            expected_version = version if windows else 'txm version '+version
            if (Path(report['archive'].replace('\\', '/')).name != archive.name
                    or report['sha256'] != digest or report['version'] != expected_version):
                raise ValueError('package report does not match archive/version')
            if not windows:
                checks = {'native_child', 'authenticated_signal', 'exit_receipt',
                          'receipt_after_coordinator_exit', 'child_metrics'}
                smoke = report.get('runtime_smoke', {})
                if set(smoke) != checks or any(value is not True for value in smoke.values()):
                    raise ValueError('incomplete packaged runtime smoke')
        elif report.get('tests_run') != 2 or report.get('skipped') != []:
            raise ValueError('foreign-UID checks must both run without skips')
        row = {key: report[key] for key in ['passed', 'version', 'runtime_smoke', 'tests_run',
                                           'platform', 'machine', 'scope', 'observations'] if key in report}
        row.update(file=name, report_sha256=hashlib.sha256(raw).hexdigest(),
                   archive=archive.name, archive_sha256=digest, source_sha=commit)
        text = json.dumps(row, separators=(',', ':'))
        if len(text.encode()) > 3000:
            raise ValueError('package proof exceeds annotation budget')
        rows.append((name, text))
    return rows


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--directory', required=True, type=Path)
    parser.add_argument('--archive', required=True, type=Path)
    parser.add_argument('--expected-version', required=True)
    parser.add_argument('--commit', required=True)
    parser.add_argument('--system', required=True, choices=['Linux', 'macOS', 'Windows'])
    args = parser.parse_args()
    for name, text in summaries(args.directory, args.archive, args.expected_version, args.commit, args.system):
        escaped = text.replace('%', '%25').replace('\r', '%0D').replace('\n', '%0A')
        print(f'::notice title=Verified package {name}::{escaped}')


if __name__ == '__main__':
    main()
