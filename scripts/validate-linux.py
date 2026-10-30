#!/usr/bin/env python3
"""Validate Linux foreign-UID, loopback SSH and real Screen/Zellij in disposable containers.

Requires explicit artifacts; never mounts host HOME, credentials or default sockets.
Builds only the test image, with network disabled for every runtime container.
"""
import argparse
import json
import os
from pathlib import Path
import platform
import secrets
import shutil
import subprocess
import sys
import time

from importlib.util import spec_from_file_location, module_from_spec

REPO = Path(__file__).resolve().parents[1]
spec = spec_from_file_location('runtime_validation', REPO/'scripts/validate-runtime.py')
runtime = module_from_spec(spec)
spec.loader.exec_module(runtime)
ZELLIJ_SHA = '0ec6ef07b63c6355c02ce18343d40ef5ef5af19e25313ea9009c8fceda29e94f'


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True, type=Path)
    parser.add_argument('--race-binary', required=True, type=Path)
    parser.add_argument('--archive', required=True, action='append', type=Path)
    parser.add_argument('--zellij', required=True, type=Path,
                        help='Verified v0.45.1 x86_64 Linux no-web release executable')
    parser.add_argument('--claude', type=Path, help='Optional explicit CLI for offline init-only hook check')
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    if sys.platform != 'linux' or platform.machine() != 'x86_64':
        parser.error('this container matrix certifies only Linux amd64')
    binaries = [('normal', args.binary.resolve(strict=True)), ('race', args.race_binary.resolve(strict=True))]
    archives = [p.resolve(strict=True) for p in args.archive]
    zellij = args.zellij.resolve(strict=True)
    if runtime.sha(zellij) != ZELLIJ_SHA:
        parser.error('Zellij must match the verified no-web v0.45.1 release binary')
    metadata = subprocess.run(['go', 'version', '-m', str(binaries[1][1])], capture_output=True, text=True,
                              env=dict(os.environ, GOTOOLCHAIN='local', GOPROXY='off'), timeout=10)
    if metadata.returncode or '\tbuild\t-race=true' not in metadata.stdout:
        parser.error('race instrumentation required')
    claude = args.claude.resolve(strict=True) if args.claude else None
    output = args.output.absolute()
    output.mkdir(mode=0o700)  # No clobbering or reuse of fixture outputs.
    reports = output/'container-reports'
    reports.mkdir()
    before = runtime.source_fingerprint()
    fixture_source = output/'fixture-source'
    (fixture_source/'tests').mkdir(parents=True)
    for path in (REPO/'tests').glob('*.py'):
        if path.is_symlink():
            raise ValueError('fixture source must be a regular file')
        shutil.copy2(path, fixture_source/'tests'/path.name)
    for i, archive in enumerate(archives):
        binaries.append(('archive-'+str(i), runtime.extract_binary(archive, output/('txm-archive-'+str(i)))))
    artifacts = [{'path': str(p), 'sha256': runtime.sha(p)} for _, p in binaries]
    artifacts += [{'path': str(p), 'sha256': runtime.sha(p)} for p in [*archives, zellij, *([claude] if claude else [])]]
    token = secrets.token_hex(12)
    label = 'txm.validation='+token
    image = 'txm-linux-validation:'+token
    report = {'schema_version': 1, 'passed': False, 'source': before, 'artifacts': artifacts,
              'host': {'kernel': platform.release(), 'architecture': platform.machine()}, 'checks': [],
              'scope': 'Container-only root/foreign UID; loopback SSH only; no host credentials/accounts/services or platform certification'}

    def save():
        (output/'report.tmp').write_text(json.dumps(report, indent=2)+'\n')
        (output/'report.tmp').replace(output/'report.json')

    def run(name, argv, timeout=900):
        started = time.time()
        code = 124
        try:
            with (output/(name+'.log')).open('xb') as log:
                code = subprocess.run(list(map(str, argv)), cwd=REPO, stdout=log,
                                      stderr=subprocess.STDOUT, timeout=timeout).returncode
        finally:
            report['checks'].append({'name': name, 'argv': list(map(str, argv)), 'exit_code': code,
                                     'started': started, 'finished': time.time()})
            save()
        if code:
            raise RuntimeError(name+' failed; inspect its log')

    def container(name, binary, command, extra_mounts=()):
        cname = 'txm-validation-'+token+'-'+name
        argv = ['docker', 'run', '--name', cname, '--label', label, '--rm', '--init',
                '--network', 'none', '--read-only', '--tmpfs', '/tmp:rw,exec,size=1g',
                '--tmpfs', '/run:rw,size=32m', '--tmpfs', '/root:rw,size=8m,mode=0700',
                '--env', 'TXM_LINUX_FIXTURE=1', '--env', 'PYTHONDONTWRITEBYTECODE=1']
        mounts = [(fixture_source, '/work', True), (binary, '/opt/txm', True), (zellij, '/opt/zellij', True),
                  (reports, '/results', False), *extra_mounts]
        for source, dest, readonly in mounts:
            if ',' in str(source):
                raise ValueError('Docker mount source cannot contain a comma')
            argv += ['--mount', 'type=bind,src='+str(source)+',dst='+dest+(',readonly' if readonly else '')]
        argv += ['--workdir', '/work', report['image_id'], *command]
        try:
            run(name, argv)
        finally:
            # A timed-out CLI may leave its container. Only remove our exact
            # random name after verifying our ownership label; never search by PID.
            p = subprocess.run(['docker', 'inspect', '--format', '{{json .Config.Labels}}', cname],
                               capture_output=True, text=True, timeout=15)
            if p.returncode == 0:
                if json.loads(p.stdout).get('txm.validation') != token:
                    raise RuntimeError('container ownership changed; refusing cleanup')
                subprocess.run(['docker', 'rm', '--force', cname], capture_output=True,
                               timeout=20, check=True)

    save()
    try:
        run('image-build', ['docker', 'build', '--label', label, '--tag', image,
                           '--file', REPO/'tests/containers/Dockerfile.linux', REPO/'tests/containers'], timeout=600)
        p = subprocess.run(['docker', 'image', 'inspect', '--format', '{{.Id}}', image],
                           capture_output=True, text=True, timeout=15, check=True)
        report['image_id'] = p.stdout.strip()
        for name, binary in binaries:
            container(name, binary, ['/usr/bin/python3', 'tests/linux_followup_checks.py', '--binary', '/opt/txm',
                                    '--zellij', '/opt/zellij', '--report', '/results/'+name+'.json', '-v'])
            result = json.loads((reports/(name+'.json')).read_text())
            report.setdefault('runtime_results', {})[name] = result
            if not result['passed'] or result['skipped'] or result['tests_run'] != 9:
                raise RuntimeError(name+' did not execute every required Linux group')
            save()
        if claude:
            container('claude-init', binaries[0][1], ['/usr/bin/python3', 'tests/agent_events_checks.py',
                      '--binary', '/opt/txm', '--real-claude', '/opt/claude',
                      'AgentEventsChecks.test_real_claude_init_only_sessionstart_no_model_or_auth', '-v'],
                      [(claude, '/opt/claude', True)])
        if runtime.source_fingerprint() != before:
            raise RuntimeError('source/harness changed during validation')
        for artifact in artifacts:
            if runtime.sha(Path(artifact['path'])) != artifact['sha256']:
                raise RuntimeError('artifact changed during validation')
        report['passed'] = True
    except Exception as e:
        report['failure'] = str(e)
    finally:
        save()
        # Delete only the unique tag we just created, and only if it still names
        # the recorded image. Layers/different tags and preexisting images remain.
        if report.get('image_id'):
            p = subprocess.run(['docker', 'image', 'inspect', '--format', '{{.Id}}', image],
                               capture_output=True, text=True, timeout=15)
            if p.returncode == 0 and p.stdout.strip() == report['image_id']:
                subprocess.run(['docker', 'image', 'rm', image], capture_output=True, timeout=20, check=True)
    print(json.dumps({'passed': report['passed'], 'report': str(output/'report.json')}))
    return 0 if report['passed'] else 1


if __name__ == '__main__':
    raise SystemExit(main())
