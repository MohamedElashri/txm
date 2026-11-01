#!/usr/bin/env python3
"""Smoke a host-compatible release ZIP and install only into a guarded HOME."""
import argparse
import hashlib
import json
from pathlib import Path
import sys
import zipfile

import agent_checks as a
from sandbox import wait_for

EXPECTED = {
    'txm', 'LICENSE', 'README.md', 'docs/txm.1',
    'completions/txm.bash', 'completions/_txm',
    'completions/txm.fish', 'completions/txm.ps1',
}


def without_lazy_help(text):
    # Cobra adds help to the currently executed command lazily. `install`
    # and `completion bash` therefore place these built-in entries differently.
    lazy = {'flags+=("--help")', 'flags+=("-h")',
            'local_nonpersistent_flags+=("--help")', 'local_nonpersistent_flags+=("-h")'}
    return '\n'.join(line for line in text.splitlines() if line.strip() not in lazy)


def runtime_smoke(box, binary):
    """Exercise packaged native code without repeating the source regression matrix."""
    a.BINARY = binary
    release = box.root / 'release-worker'
    worker = box.root / 'package-worker.py'
    worker.write_text('''from pathlib import Path
import subprocess,sys,time
subprocess.run([sys.argv[1], 'agent', 'signal', '--from-env', '--state', 'turn-complete'], check=True)
deadline=time.monotonic()+15
while not Path(sys.argv[2]).exists() and time.monotonic()<deadline: pass
''')
    with a.coordinator(box):
        job = a.start(box, 'package-smoke', [sys.executable, str(worker), str(binary), str(release)],
                      extra=['--provider', 'generic', '--metrics', '--metrics-interval', '2'])
        def sampled():
            metrics = a.envelope(box, ['agent', 'metrics', job['id']])['data']
            latest = metrics.get('latest') or {}
            return (latest.get('state') == 'available' and latest.get('scope') == 'child_pid'
                    and latest.get('start_identity') and (latest.get('rss_bytes') or 0) > 0
                    and (latest.get('cpu_percent') or 0) > 0)
        wait_for(sampled, timeout=10)
        active = a.status(box, job['id'])
        assert active['lifecycle'] == 'running' and active['turn_complete_at']
        release.touch()
        ended = a.envelope(box, ['agent', 'wait', job['id'], '--until', 'exited',
                                 '--timeout', '10s', '--fail-on-nonzero'])['data']['agent']
        receipt = ended['receipt']
        assert ended['lifecycle'] == 'exited' and receipt['exit_code'] == 0
        assert receipt['instance_id'] == job['instance_id'] and receipt['managed_id'] == job['id']
    recovered = a.envelope(box, ['--backend', 'native', 'agent', 'status', job['id'], '--existing'])['data']['agent']
    assert recovered['receipt'] == receipt
    assert not (a.runtime(box) / 'coordinator.json').exists()
    return dict.fromkeys(['native_child', 'authenticated_signal', 'exit_receipt',
                          'receipt_after_coordinator_exit', 'child_metrics'], True)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--archive', type=Path, required=True)
    parser.add_argument('--report', type=Path, required=True)
    parser.add_argument('--expected-version', help='Require the packaged version stamp to match this release')
    args = parser.parse_args()
    archive = args.archive.resolve(strict=True)
    with a.fixture() as box, zipfile.ZipFile(archive) as payload:
        names = payload.namelist()
        assert len(names) == len(EXPECTED) and set(names) == EXPECTED, names
        for name in names:
            info = payload.getinfo(name)
            assert 0 < info.file_size <= 100 * 1024 * 1024, name
            # Write only the known relative paths; do not trust ZIP paths/modes.
            dest = box.root / 'payload' / name
            dest.parent.mkdir(parents=True, exist_ok=True)
            dest.write_bytes(payload.read(name))
        binary = box.root / 'payload/txm'
        binary.chmod(0o700)
        version = box.run([binary, 'version']).stdout.strip()
        if args.expected_version:
            assert version == 'txm version '+args.expected_version, version
        box.run([binary, 'install'])
        home = box.root / 'home'
        installed = home / '.local/bin/txm'
        assert installed.read_bytes() == binary.read_bytes()
        assert (home / '.local/share/man/man1/txm.1').read_bytes() == payload.read('docs/txm.1')
        assert box.run([installed, 'version']).stdout.strip() == version
        flags = box.run([binary, '__complete', 'profile', '--']).stdout
        assert '--dry-run' in flags and '--attach-existing' in flags
        agent_flags = box.run([binary, '__complete', 'agent', 'start', '--']).stdout
        assert '--cwd' in agent_flags and '--env' in agent_flags and '--provider' in agent_flags
        event_flags=box.run([binary,'__complete','events','--']).stdout
        assert '--jsonl' in event_flags and '--id' in event_flags
        wait_flags=box.run([binary,'__complete','agent','wait','--']).stdout
        assert '--until' in wait_flags and '--fail-on-nonzero' in wait_flags
        dashboard_flags=box.run([binary,'__complete','dashboard','--']).stdout
        assert '--agents' in dashboard_flags and '--preview' in dashboard_flags and '--refresh' in dashboard_flags
        task_flags=box.run([binary,'__complete','task','start','--']).stdout
        assert '--repo' in task_flags and '--dry-run' in task_flags and '--allow-repo-commands' in task_flags
        remote_flags=box.run([binary,'__complete','remote','attach','--']).stdout
        assert '--host-id' in remote_flags and '--session-id' in remote_flags and '--read-only' in remote_flags
        for shell, relative in [('bash', '.local/share/bash-completion/completions/txm'),
                                ('zsh', '.zfunc/_txm'),
                                ('fish', '.config/fish/completions/txm.fish')]:
            generated = box.run([binary, 'completion', shell]).stdout
            assert without_lazy_help((home / relative).read_text()) == without_lazy_help(generated), shell
            packaged = {'bash': 'txm.bash', 'zsh': '_txm', 'fish': 'txm.fish'}[shell]
            assert payload.read('completions/' + packaged).decode() == generated, shell
        assert payload.read('completions/txm.ps1').decode() == box.run([binary, 'completion', 'powershell']).stdout
        assert not (box.root / 'runtime/txm').exists()
        assert not (box.root / 'state/txm').exists()
        smoke = runtime_smoke(box, installed)
    report = {'archive': str(archive), 'sha256': hashlib.sha256(archive.read_bytes()).hexdigest(),
              'files': sorted(EXPECTED), 'version': version,
              'passed': True, 'runtime_smoke': smoke,
              'scope': 'host-compatible ZIP, private install and native child/signal/receipt/metrics smoke; no system install/publication'}
    args.report.write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report))


if __name__ == '__main__':
    main()
