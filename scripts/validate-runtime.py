#!/usr/bin/env python3
"""Run the guarded runtime matrix and bind its outputs to exact source/artifacts.

No build/download, real host, provider CLI, publication or real session discovery.
Use a new output directory. Missing required tools are blockers, not passes.
"""
import argparse
from concurrent.futures import ThreadPoolExecutor
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import sys
import threading
import time
import zipfile

REPO = Path(__file__).resolve().parents[1]
SUITES = ('cli_errors', 'native_checks', 'structured_checks',
          'profile_checks', 'agent_checks', 'agent_events_checks',
          'dashboard_checks', 'extension_checks', 'prune_checks')


def sha(path):
    h = hashlib.sha256()
    with path.open('rb') as f:
        for block in iter(lambda: f.read(1 << 20), b''):
            h.update(block)
    return h.hexdigest()


def source_fingerprint(repo=REPO):
    # Include the harness and this runner; omit mutable documentation/reports and caches.
    paths = [*repo.joinpath('pkg').rglob('*.go'), *repo.joinpath('tests').glob('*.py'),
             *repo.joinpath('scripts').glob('*'), *repo.joinpath('tests/containers').glob('*'),
             *repo.joinpath('.github/workflows').glob('*.yml'), *repo.joinpath('.github/scripts').glob('*.sh'), repo/'Makefile', repo/'VERSION',
             repo/'.goreleaser.yaml', repo/'.gitattributes', repo/'.gitignore', repo/'main.go', repo/'go.mod',
             repo/'go.sum', repo/'docs/embeds.go', repo/'docs/txm.1']
    paths = sorted(set(p for p in paths if p.is_file()))
    h = hashlib.sha256()
    for p in paths:
        h.update(p.relative_to(repo).as_posix().encode()+b'\0'+p.read_bytes()+b'\0')
    return {'sha256': h.hexdigest(), 'files': [p.relative_to(repo).as_posix() for p in paths]}


def extract_binary(archive, dest):
    # The install test separately validates the full archive allowlist.
    with zipfile.ZipFile(archive) as payload:
        matches = [i for i in payload.infolist() if i.filename == 'txm']
        if len(matches) != 1 or not 0 < matches[0].file_size <= 100 << 20:
            raise ValueError('archive must contain exactly one bounded txm binary')
        dest.write_bytes(payload.read(matches[0]))
    dest.chmod(0o700)
    return dest


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True, type=Path)
    parser.add_argument('--race-binary', type=Path)
    parser.add_argument('--archive', action='append', type=Path, default=[])
    parser.add_argument('--macos-backends', action='store_true',
                        help='Also require and test installed macOS Screen/Zellij in private namespaces')
    parser.add_argument('--loopback-ssh', action='store_true',
                        help='Also require real unprivileged macOS loopback SSH/PTY checks')
    parser.add_argument('--expected-version', help='Require the archive version stamp to match this release')
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    # Fail before creating output or executing any test when isolation prerequisites
    # are absent. Never count adapter skips as the full native/tmux gate.
    if sys.platform not in ('linux', 'darwin'):
        parser.error('guarded runtime matrix requires Linux or macOS')
    for name in ('git', 'tmux'):
        if shutil.which(name) is None:
            parser.error(name+' is required for the full guarded matrix')
    if args.macos_backends:
        if sys.platform != 'darwin':
            parser.error('--macos-backends requires actual macOS')
        for name in ('screen', 'zellij'):
            if shutil.which(name) is None:
                parser.error(name+' is required for --macos-backends')
    if args.loopback_ssh:
        if sys.platform != 'darwin' or os.geteuid() == 0:
            parser.error('--loopback-ssh requires unprivileged macOS')
        for path in ('/usr/sbin/sshd', '/usr/bin/ssh', '/usr/bin/ssh-keygen'):
            if not os.access(path, os.X_OK):
                parser.error(path+' is required for --loopback-ssh')
    binaries = [('normal', args.binary.resolve(strict=True))]
    if args.race_binary:
        binaries.append(('race', args.race_binary.resolve(strict=True)))
    for _, binary in binaries:
        if not binary.is_file() or not os.access(binary, os.X_OK):
            parser.error('binary must be an existing executable')
    if args.race_binary:
        go = shutil.which('go')
        if go is None:
            parser.error('go is required to verify race instrumentation')
        metadata = subprocess.run([go, 'version', '-m', str(binaries[1][1])],
                                  env=dict(os.environ, GOTOOLCHAIN='local', GOPROXY='off'),
                                  capture_output=True, text=True, timeout=10)
        if metadata.returncode or '\tbuild\t-race=true' not in metadata.stdout:
            parser.error('--race-binary must contain verified Go race instrumentation')
    archives = [p.resolve(strict=True) for p in args.archive]
    output = args.output.absolute()
    output.mkdir(mode=0o700)  # Refuse existing dirs/symlinks and accidental log clobber.
    before = source_fingerprint()
    for i, archive in enumerate(archives):
        binaries.append(('archive-'+str(i), extract_binary(archive, output/('txm-archive-'+str(i)))))
    report = {'schema_version': 1, 'scope': platform.system()+' guarded fixtures; not a whole-project release certification',
              'host': {'os': platform.system(), 'architecture': platform.machine(),
                       'kernel': platform.release(), 'python': platform.python_version()},
              'source': before, 'artifacts': [], 'checks': [], 'passed': False,
              'skip_notes': ['Real provider initialization is optional and not invoked here.',
                             ('Actual private macOS loopback SSH enabled; no WAN/inter-OS claim.' if args.loopback_ssh else
                              'Fake SSH only; actual SSH server/network gate remains separate.'),
                             'Only the recorded host OS/architecture is runtime validated; other platforms remain open.']}
    for label, binary in binaries:
        report['artifacts'].append({'label': label, 'path': str(binary), 'sha256': sha(binary)})
    for archive in archives:
        report['artifacts'].append({'label': 'zip', 'path': str(archive), 'sha256': sha(archive)})
    lock = threading.Lock()

    def save():
        temp = output/'report.tmp'
        temp.write_text(json.dumps(report, indent=2)+'\n')
        temp.replace(output/'report.json')

    def run(name, argv):
        started = time.time()
        with (output/(name+'.log')).open('xb') as log:
            try:
                p = subprocess.run(argv, cwd=REPO, stdout=log, stderr=subprocess.STDOUT, timeout=900)
                code = p.returncode
            except subprocess.TimeoutExpired:
                # Do not guess a process identity for cleanup; inspect the owned
                # fixture if a trusted harness ever exceeds its internal deadlines.
                code = 124
        with lock:
            report['checks'].append({'name': name, 'argv': list(map(str, argv)),
                                     'exit_code': code, 'started': started, 'finished': time.time()})
            save()
        if code:
            raise RuntimeError(name+' failed; inspect its log and any owned fixture')

    def flavor(item):
        label, binary = item
        for suite in (*SUITES, *(['macos_backend_checks'] if args.macos_backends else []),
                      *(['ssh_checks'] if args.loopback_ssh else [])):
            argv = [sys.executable, str(REPO/'tests'/(suite+'.py')), '--binary', str(binary)]
            if suite in ('dashboard_checks', 'macos_backend_checks', 'ssh_checks'):
                argv += ['--report', str(output/(label+'-'+suite+'.json'))]
            if suite in ('agent_checks', 'agent_events_checks', 'dashboard_checks', 'extension_checks', 'prune_checks', 'macos_backend_checks', 'ssh_checks'):
                argv += ['-v']
            run(label+'-'+suite, argv)

    save()
    try:
        run('harness', [sys.executable, '-m', 'unittest', 'discover', '-s', 'tests', '-p', 'test_*.py', '-v'])
        for i, archive in enumerate(archives):
            run('archive-'+str(i)+'-install', [sys.executable, str(REPO/'tests/archive_checks.py'),
                 '--archive', str(archive), '--report', str(output/('archive-'+str(i)+'-install.json')),
                 *(['--expected-version', args.expected_version] if args.expected_version else [])])
        with ThreadPoolExecutor(max_workers=2) as pool:
            # Consume every result; failures must not disappear in worker futures.
            futures = [pool.submit(flavor, item) for item in binaries]
            failures = []
            for future in futures:
                try:
                    future.result()
                except Exception as e:
                    failures.append(str(e))
            if failures:
                raise RuntimeError('; '.join(failures))
        # Measure performance alone, after all concurrent runtime workers finish.
        run('performance', [sys.executable, str(REPO/'tests/performance.py'),
             '--binary', str(binaries[0][1]), '--report', str(output/'performance.json')])
        if source_fingerprint() != before:
            raise RuntimeError('source/harness changed during validation')
        for artifact in report['artifacts']:
            if sha(Path(artifact['path'])) != artifact['sha256']:
                raise RuntimeError('artifact changed during validation')
        report['passed'] = True
    except Exception as e:
        report['failure'] = str(e)
    finally:
        save()
    print(json.dumps({'passed': report['passed'], 'report': str(output/'report.json')}))
    return 0 if report['passed'] else 1


if __name__ == '__main__':
    raise SystemExit(main())
