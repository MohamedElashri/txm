#!/usr/bin/env python3
"""Require trusted staged CI and exact-source validated release artifacts."""
import argparse
import json
import re
import subprocess
from pathlib import Path

REQUIRED_JOBS = {'Scope', 'Lint', 'Tests', 'Build / Linux + Windows',
                 'Build / macOS universal', 'Validate / Linux', 'Validate / macOS',
                 'Validate / Archives', 'Validate package / Linux x86_64 GNU',
                 'Validate package / Linux x86_64 musl', 'Validate package / Linux ARM64 GNU',
                 'Validate package / Linux ARM64 musl', 'Validate package / macOS universal',
                 'Validate package / Windows x86_64', 'Validate package / Windows ARM64', 'CI ready'}
RELEASE_ARTIFACTS = ('ci-release-bundle', 'ci-release-evidence')


def select_run(runs, repository, sha, branch, manual=False):
    eligible = []
    for run in runs:
        trusted_event = (run.get('event') == 'push' and branch == 'main' or
                         (branch == 'main' or manual) and run.get('event') == 'workflow_dispatch')
        if (run.get('head_sha') == sha and run.get('head_branch') == branch
                and run.get('repository', {}).get('full_name') == repository
                and run.get('path') == '.github/workflows/ci.yml' and trusted_event):
            eligible.append(run)
    if not eligible:
        raise ValueError('No successful CI for this exact commit and branch. '
                         'Finish/rerun CI first, then retry release.')
    latest = max(eligible, key=lambda run: (run['created_at'], run['id']))
    if latest.get('status') != 'completed' or latest.get('conclusion') != 'success':
        raise ValueError('Latest exact-commit CI is not successful. Finish/rerun CI, then retry release.')
    return latest


def verify_jobs(jobs):
    names = [job['name'] for job in jobs]
    if (not REQUIRED_JOBS.issubset(names) or len(names) != len(set(names))
            or any(job.get('status') != 'completed' or job.get('conclusion') != 'success' for job in jobs)):
        raise ValueError('Every lint, test, OS build, validation and CI-ready stage must pass; skipped jobs do not qualify')


def select_release_artifacts(artifacts, sha):
    result = {}
    for name in RELEASE_ARTIFACTS:
        found = [item for item in artifacts if item.get('name') == name]
        if (len(found) != 1 or found[0].get('expired') is not False
                or found[0].get('workflow_run', {}).get('head_sha') != sha
                or not re.fullmatch(r'sha256:[0-9a-f]{64}', found[0].get('digest') or '')
                or found[0].get('size_in_bytes', 0) <= 0):
            raise ValueError('Exact-commit bundle/evidence artifact is missing, expired or invalid; rerun CI validation before release')
        result[name] = found[0]
    return result


def api(endpoint):
    return json.loads(subprocess.check_output(['gh', 'api', endpoint], text=True, timeout=30))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repository', required=True)
    parser.add_argument('--sha', required=True)
    parser.add_argument('--branch', default='main')
    parser.add_argument('--manual', action='store_true', help='Also accept manually dispatched CI on this branch')
    parser.add_argument('--report', type=Path, required=True)
    args = parser.parse_args()
    if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', args.repository) or not re.fullmatch(r'[0-9a-f]{40}', args.sha):
        parser.error('expected owner/repository and a complete commit SHA')
    try:
        runs = api(f'repos/{args.repository}/actions/workflows/ci.yml/runs?head_sha={args.sha}&per_page=100')['workflow_runs']
        run = select_run(runs, args.repository, args.sha, args.branch, args.manual)
        # The API returns the latest attempt, including rerun failed jobs.
        jobs = api(f'repos/{args.repository}/actions/runs/{run["id"]}/jobs?filter=latest&per_page=100')['jobs']
        verify_jobs(jobs)
        artifacts = select_release_artifacts(api(f'repos/{args.repository}/actions/runs/{run["id"]}/artifacts?per_page=100')['artifacts'], args.sha)
        report = {'passed': True, 'commit': args.sha, 'ci_run_id': run['id'],
                  'ci_run_attempt': run['run_attempt'], 'url': run['html_url'],
                  'release_artifacts': {name: {key: item[key] for key in ['id', 'name', 'digest', 'size_in_bytes']}
                                        for name, item in artifacts.items()}}
        with args.report.open('x') as output:
            output.write(json.dumps(report, indent=2)+'\n')
    except (OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
        parser.exit(1, str(error)+'\n')
    print(json.dumps(report))


if __name__ == '__main__':
    main()
