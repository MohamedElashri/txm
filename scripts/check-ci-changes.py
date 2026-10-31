#!/usr/bin/env python3
"""Skip expensive automatic CI only when every changed path is documentation."""
import json
import os
from pathlib import Path
import re
import subprocess


def needs_full_ci(paths):
    # docs/ also contains Go code: never exempt the entire directory.
    return any(not (path.endswith('.md') or path == 'docs/txm.1') for path in paths)


def select_scope(event_name, event, force=False):
    if force or event_name not in ('push', 'pull_request'):
        return True
    try:
        if event_name == 'pull_request':
            base = event['pull_request']['base']['sha']
            head = event['pull_request']['head']['sha']
            separator = '...'
        else:
            base, head = event['before'], event['after']
            separator = '..'
        if any(not isinstance(sha, str) or not re.fullmatch(r'[0-9a-f]{40}', sha)
               or sha == '0'*40 for sha in (base, head)):
            return True
        # Disabling rename detection exposes both paths of a code-to-doc rename.
        result = subprocess.run(
            ['git', 'diff', '--no-renames', '--name-only', '-z', base+separator+head, '--'],
            check=True, capture_output=True, timeout=30)
        paths = result.stdout.decode('utf-8', errors='surrogateescape').split('\0')
        return needs_full_ci(path for path in paths if path)
    except (KeyError, TypeError, OSError, subprocess.SubprocessError):
        # Missing history or an unfamiliar payload must not suppress validation.
        return True


def main():
    event_path = os.environ.get('GITHUB_EVENT_PATH')
    try:
        event = json.loads(Path(event_path).read_text()) if event_path else {}
    except (OSError, ValueError):
        event = {}
    full = select_scope(os.environ.get('GITHUB_EVENT_NAME'), event,
                        os.environ.get('FORCE_FULL_CI') == 'true')
    value = str(full).lower()
    with Path(os.environ['GITHUB_OUTPUT']).open('a') as output:
        output.write('full_ci='+value+'\n')
    print('Full CI required' if full else 'Documentation-only changes: skipping expensive jobs')


if __name__ == '__main__':
    main()
