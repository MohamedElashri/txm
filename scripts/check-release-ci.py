#!/usr/bin/env python3
"""Require successful CI for the exact commit before spending release minutes."""
import argparse
import json
import re
import subprocess
from pathlib import Path

REQUIRED_JOBS = {'build-and-test', 'lint', 'macos-integration (macos-15)',
                 'macos-integration (macos-15-intel)'}


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
                         'Finish/rerun CI first, then rerun the failed release jobs; no build has started.')
    latest = max(eligible, key=lambda run: (run['created_at'], run['id']))
    if latest.get('status') != 'completed' or latest.get('conclusion') != 'success':
        raise ValueError('Latest exact-commit CI is not successful. Finish/rerun CI, then rerun release; no build has started.')
    return latest


def verify_jobs(jobs):
    names = [job['name'] for job in jobs]
    if (not REQUIRED_JOBS.issubset(names) or len(names) != len(set(names))
            or any(job.get('status') != 'completed' or job.get('conclusion') != 'success' for job in jobs)):
        raise ValueError('CI must include successful Linux, macOS ARM64/Intel and lint jobs; skipped jobs do not qualify')


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
        report = {'passed': True, 'commit': args.sha, 'ci_run_id': run['id'],
                  'ci_run_attempt': run['run_attempt'], 'url': run['html_url']}
        with args.report.open('x') as output:
            output.write(json.dumps(report, indent=2)+'\n')
    except (OSError, ValueError, KeyError, subprocess.SubprocessError) as error:
        parser.exit(1, str(error)+'\n')
    print(json.dumps(report))


if __name__ == '__main__':
    main()
