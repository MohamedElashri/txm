#!/usr/bin/env python3
"""real PTY, immutable receipt and coordinator recovery regressions."""
import argparse
from concurrent.futures import ThreadPoolExecutor
from contextlib import contextmanager
import json
import os
from pathlib import Path
import secrets
import select
import shutil
import signal
import socket
import struct
import subprocess
import sys
import time
import unittest

from sandbox import Sandbox, native_server, wait_for
from structured_checks import fake, calls, tmux
from wire import Peer, exact

BINARY = None


def runtime(box):
    return box.root / 'runtime/txm/v1/agents'


def state(box):
    return box.root / 'state/txm/v1/agents'


def envelope(box, args, code=0):
    out = box.run([BINARY, '--json', *args], check=False, timeout=22)
    if out.returncode != code:
        raise AssertionError((args[0:3], out.returncode, out.stdout, out.stderr))
    value = json.loads(out.stdout)
    assert value['schema_version'] == 1
    assert not out.stderr, out.stderr
    return value


def start(box, name, argv, backend='native', extra=None):
    return envelope(box, ['--backend', backend, 'agent', 'start', *(extra or []), name, '--', *argv])['data']['agent']


def status(box, ident):
    return envelope(box, ['agent', 'status', ident])['data']['agent']


def await_exit(box, ident, timeout=8):
    result = {}
    def done():
        result.update(status(box, ident))
        return result['lifecycle'] == 'exited'
    wait_for(done, timeout=timeout)
    return result


@contextmanager
def coordinator(box, binary=None):
    epoch = secrets.token_hex(16)
    log = box.root / ('coordinator-' + epoch + '.stderr')
    with log.open('wb') as output:
        proc = box.start([binary or BINARY, 'internal', 'coordinator', '--epoch', epoch],
                         stdout=subprocess.DEVNULL, stderr=output)
    def ready():
        if proc.poll() is not None:
            raise AssertionError('coordinator startup failed: ' + log.read_text())
        path = runtime(box) / 'coordinator.json'
        return path.exists() and json.loads(path.read_text())['epoch'] == epoch
    try:
        wait_for(ready)
        yield proc
    finally:
        if proc.poll() is None:
            proc.terminate()
            proc.wait(timeout=18)
        data = log.read_text()
        if proc.returncode == 66 or 'WARNING: DATA RACE' in data:
            raise AssertionError(data)


@contextmanager
def fixture():
    with Sandbox() as box:
        box.env['NOT_ALLOWED'] = 'PRIVATE_ENV_VALUE'
        box.env['GORACE'] = 'log_path=' + str(box.root / 'race') + ' atexit_sleep_ms=0 exitcode=66'
        try:
            yield box
        finally:
            box.guard()
            # Only native instances in this fixture's private runtime. Each
            # control handshake is bound to its random filename instance.
            for path in (box.root / 'runtime/txm/v1').glob('s-*.sock'):
                try:
                    with Peer(path, 'control') as peer:
                        assert peer.identity['instance_id'] == path.stem[2:]
                        peer.request('kill')
                    wait_for(lambda: not path.exists(), timeout=6)
                except (ConnectionRefusedError, FileNotFoundError, EOFError):
                    pass
            reports = list(box.root.glob('race.*'))
            if reports:
                raise AssertionError('\n'.join(p.read_text() for p in reports))


def rpc(box, body, fragmented=False):
    metadata = json.loads((runtime(box) / 'coordinator.json').read_text())
    path = runtime(box) / ('s-' + metadata['epoch'] + '.sock')
    if isinstance(body, dict):
        host = json.loads((state(box).parent / 'host.json').read_text())['host_id']
        body = json.dumps({'schema_version': 1, 'epoch': metadata['epoch'], 'host_id': host, 'state_root': str(state(box)), **body}).encode()
    packet = struct.pack('!I', len(body)) + body
    with socket.socket(socket.AF_UNIX) as conn:
        conn.settimeout(18)
        conn.connect(str(path))
        if fragmented:
            for byte in packet:
                conn.sendall(bytes([byte]))
        else:
            conn.sendall(packet)
        size = struct.unpack('!I', exact(conn, 4))[0]
        assert size <= 1048576
        return json.loads(exact(conn, size))


class AgentChecks(unittest.TestCase):
    def test_human_stop_requires_exact_confirmation_and_backend(self):
        with fixture() as box, coordinator(box):
            job = start(box, 'human-stop', ['/bin/sleep', '30'])
            registry = state(box) / 'registry.json'
            before = registry.read_bytes()
            envelope(box, ['agent', 'stop', job['id'], '--confirm', 'f'*32], code=2)
            self.assertEqual(registry.read_bytes(), before)
            self.assertEqual(status(box, job['id'])['lifecycle'], 'running')
            envelope(box, ['--backend', 'tmux', 'agent', 'stop', job['id'], '--confirm', job['id']], code=2)
            stopped = envelope(box, ['agent', 'stop', job['id'], '--confirm', job['id']])['data']['agent']
            self.assertEqual(stopped['lifecycle'], 'exited'); self.assertIsNotNone(stopped['receipt'])
            repeated = envelope(box, ['agent', 'stop', job['id'], '--confirm', job['id']])['data']['agent']
            self.assertEqual(repeated['receipt'], stopped['receipt'])

    def test_different_state_identity_cannot_use_running_coordinator(self):
        with fixture() as box, coordinator(box) as service:
            envelope(box, ['agent', 'list'])
            before = (state(box) / 'registry.json').read_bytes()
            alternate = box.root / 'alternate-state'; alternate.mkdir(mode=0o700)
            env = dict(box.env, XDG_STATE_HOME=str(alternate))
            for args in [['agent', 'list'], ['agent', 'start', 'wrong-state', '--', '/usr/bin/true'], ['agent', 'stop-coordinator']]:
                out = box.run([BINARY, '--json', *args], env=env, check=False, timeout=20)
                self.assertEqual(out.returncode, 5, out.stdout + out.stderr)
                self.assertEqual(json.loads(out.stdout)['errors'][0]['code'], 'conflict')
                # Copying the host ID still cannot redirect the running writer
                # to a different state location selected by this client.
                (alternate / 'txm/v1/host.json').write_bytes((state(box).parent / 'host.json').read_bytes())
            self.assertEqual((state(box) / 'registry.json').read_bytes(), before)
            self.assertFalse((alternate / 'txm/v1/agents').exists()); self.assertIsNone(service.poll())
    @unittest.skipUnless(sys.platform.startswith('linux') and hasattr(os, 'pidfd_open'), 'Linux pidfd ownership fixture')
    def test_concurrent_automatic_start_and_explicit_stop(self):
        with fixture() as box:
            handle = None
            try:
                with ThreadPoolExecutor(max_workers=4) as pool:
                    results = list(pool.map(lambda _: envelope(box, ['agent', 'list']), range(4)))
                self.assertTrue(all(r['data']['agents'] == [] for r in results))
                meta = json.loads((runtime(box) / 'coordinator.json').read_text())
                sock = runtime(box) / ('s-' + meta['epoch'] + '.sock')
                with socket.socket(socket.AF_UNIX) as conn:
                    conn.connect(str(sock))
                    pid = struct.unpack('3i', conn.getsockopt(socket.SOL_SOCKET, socket.SO_PEERCRED, 12))[0]
                    handle = os.pidfd_open(pid)
                    self.assertEqual(Path(f'/proc/{pid}/cwd').resolve(), box.root / 'repo')
                    self.assertIn(b'internal\0coordinator\0', Path(f'/proc/{pid}/cmdline').read_bytes())
                live = start(box, 'continues', ['/bin/sleep', '15'])
                self.assertEqual(live['lifecycle'], 'running')
                envelope(box, ['agent', 'stop-coordinator'])
                wait_for(lambda: not sock.exists())
                poll = select.poll(); poll.register(handle, select.POLLIN)
                wait_for(lambda: bool(poll.poll(0)))
                self.assertFalse((runtime(box) / 'coordinator.json').exists())
                self.assertIn('continues', box.run([BINARY, 'list']).stdout)
            finally:
                if handle is not None:
                    # Authenticated private socket + live pidfd, never a stale
                    # PID from a file. Does not target any external service.
                    try: signal.pidfd_send_signal(handle, signal.SIGTERM)
                    except ProcessLookupError: pass
                    os.close(handle)

    def test_ordinary_commands_do_not_start_or_contact_coordinator(self):
        with fixture() as box:
            with native_server(box, BINARY):
                for args in [['version'], ['completion', 'bash'], ['list'], ['list', '--json'],
                             ['backend', 'capabilities', '--backend', 'native', '--json']]:
                    box.run([BINARY, *args])
                self.assertFalse(runtime(box).exists())
                self.assertFalse(state(box).exists())
            with coordinator(box) as proc:
                proc.terminate(); proc.wait(timeout=5)
            metadata = runtime(box) / 'coordinator.json'
            metadata.write_text('corrupt coordinator sentinel'); metadata.chmod(0o600)
            box.run([BINARY, 'list', '--json'])
            self.assertEqual(metadata.read_text(), 'corrupt coordinator sentinel')

    def test_preflight_unsupported_and_invalid_never_starts_service(self):
        with fixture() as box:
            logs = [fake(box, backend, "raise AssertionError('must not mutate')") for backend in ('screen', 'zellij')]
            for backend in ('screen', 'zellij'):
                envelope(box, ['--backend', backend, 'agent', 'start', 'bad', '--', '/usr/bin/true'], 4)
            for args in [['agent', 'start', 'bad', '/usr/bin/true'],
                         ['agent', 'start', '--cwd', '/does-not-exist', 'bad', '--', '/usr/bin/true'],
                         ['agent', 'start', '--env', 'A=PRIVATE_SYNTHETIC', 'bad', '--', '/usr/bin/true'],
                         ['agent', 'start', 'bad', '--', '/does-not-exist'],
                         ['agent', 'status', 'not-an-id']]:
                out = envelope(box, args, 2)
                self.assertNotIn('PRIVATE_SYNTHETIC', json.dumps(out))
            self.assertTrue(all(not calls(log) for log in logs))
            self.assertFalse(runtime(box).exists()); self.assertFalse(state(box).exists())

    def test_native_argv_cwd_environment_and_short_exit_receipts(self):
        with fixture() as box, coordinator(box):
            cwd = box.root / 'cwd with quotes \' and spaces'; cwd.mkdir()
            script = box.root / 'child.py'; result = box.root / 'result.json'
            script.write_text('import os,json,sys\nfrom pathlib import Path\nPath(sys.argv[1]).write_text(json.dumps({"argv":sys.argv[2:],"cwd":os.getcwd(),"env":dict(os.environ),"tty":os.isatty(0),"foreground":os.tcgetpgrp(0)==os.getpgrp()}))\nraise SystemExit(42)\n')
            box.env['NOT_ALLOWED'] = 'PRIVATE_ENV_VALUE'
            args = ['Enter', '-leading', '$(touch forbidden)', ';', 'quotes \' "', 'λ', 'PRIVATE_ARG_VALUE']
            view = start(box, 'quick', [sys.executable, str(script), str(result), *args], extra=['--cwd', str(cwd)])
            view = await_exit(box, view['id'])
            self.assertEqual(view['receipt']['exit_code'], 42)
            self.assertNotEqual(view['managed_id'], view['instance_id'])
            self.assertEqual(view['managed_id'], view['id']); self.assertEqual(view['attention'], 'unknown')
            observed = json.loads(result.read_text()); self.assertEqual(observed['argv'], args)
            self.assertEqual(observed['cwd'], str(cwd)); self.assertNotIn('NOT_ALLOWED', observed['env'])
            self.assertTrue(observed['tty']); self.assertTrue(observed['foreground'])
            allowed = box.root / 'allowed.json'
            env_view = start(box, 'allowed', [sys.executable, str(script), str(allowed)], extra=['--env', 'NOT_ALLOWED'])
            await_exit(box, env_view['id'])
            self.assertEqual(json.loads(allowed.read_text())['env']['NOT_ALLOWED'], 'PRIVATE_ENV_VALUE')
            self.assertFalse(list(state(box).glob('m-*.json')))
            registry = (state(box) / 'registry.json').read_text()
            for secret in ('PRIVATE_ENV_VALUE', 'PRIVATE_ARG_VALUE', '$(touch forbidden)'):
                self.assertNotIn(secret, registry)
            for path in state(box).glob('*'):
                self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            for code in (0, 42):
                view = start(box, 'exit-' + str(code), ['/bin/sh', '-c', 'exit ' + str(code)])
                self.assertEqual(await_exit(box, view['id'])['receipt']['exit_code'], code)

    def test_parent_exit_coordinator_crash_and_receipt_recovery_no_reexecution(self):
        with fixture() as box:
            script = box.root / 'once.py'; count = box.root / 'count'; release = box.root / 'release'
            script.write_text(f'from pathlib import Path\nimport time\np=Path({str(count)!r});p.write_text(p.read_text()+"x" if p.exists() else "x")\nwhile not Path({str(release)!r}).exists(): time.sleep(.02)\nraise SystemExit(42)\n')
            with coordinator(box) as service:
                v = start(box, 'survivor', [sys.executable, str(script)])
                self.assertEqual(v['lifecycle'], 'running'); wait_for(count.exists)
                os.killpg(service.pid, signal.SIGKILL); service.wait(timeout=5)
                box.run([BINARY, 'list'])
                self.assertEqual(count.read_text(), 'x')
            with coordinator(box):
                self.assertEqual(status(box, v['id'])['lifecycle'], 'running')
                self.assertEqual(count.read_text(), 'x')
            # Child exits while the state owner is stopped; receipt survives.
            release.touch()
            wait_for(lambda: (state(box) / ('r-' + v['instance_id'] + '.json')).exists())
            with coordinator(box):
                recovered = status(box, v['id'])
                self.assertEqual(recovered['receipt']['exit_code'], 42)
                self.assertEqual(count.read_text(), 'x')
                self.assertEqual(recovered['id'], v['id'])
                self.assertEqual(recovered['instance_id'], v['instance_id'])

    def test_all_receipt_outcomes_remain_queryable_after_restart(self):
        with fixture() as box:
            backends = ['native']
            if shutil.which('tmux'):
                box.start_tmux(shutil.which('tmux')); backends.append('tmux')
            observed = []
            with coordinator(box):
                for backend in backends:
                    for label, argv in [('zero', ['/usr/bin/true']), ('nonzero', ['/bin/sh', '-c', 'exit 42']),
                                        ('signal', [sys.executable, '-c', 'import os,signal;os.kill(os.getpid(),signal.SIGUSR1)'])]:
                        view = start(box, backend + '-' + label, argv, backend=backend)
                        observed.append(await_exit(box, view['id']))
            with coordinator(box):
                for before in observed:
                    after = status(box, before['id'])
                    self.assertEqual(after['receipt'], before['receipt'])
                    self.assertEqual(after['instance_id'], before['instance_id'])
                    self.assertEqual(after['lifecycle'], 'exited')

    def test_signal_receipt_and_resistant_descendant_cleanup(self):
        with fixture() as box, coordinator(box):
            script = box.root / 'signal.py'; ready = box.root / 'ready'
            script.write_text(f'import os,signal,time\nfrom pathlib import Path\npid=os.fork()\nif pid==0:\n signal.signal(signal.SIGTERM,signal.SIG_IGN)\n while True:time.sleep(.1)\nPath({str(ready)!r}).write_text(str(pid))\nwhile True:time.sleep(.1)\n')
            v = start(box, 'signal', [sys.executable, str(script)])
            wait_for(ready.exists)
            box.run([BINARY, 'delete', 'signal'], timeout=8)
            ended = await_exit(box, v['id'])
            self.assertEqual(ended['receipt']['outcome'], 'signaled')
            self.assertIsNone(ended['receipt']['exit_code'])
            self.assertIsNotNone(ended['receipt']['signal'])
            if sys.platform.startswith('linux'):
                path = Path('/proc') / ready.read_text() / 'stat'
                wait_for(lambda: not path.exists() or path.read_text().split(') ', 1)[1].startswith('Z'))

    def test_unobserved_wrapper_loss_is_unknown(self):
        with fixture() as box, coordinator(box):
            v = start(box, 'lost', ['/bin/sleep', '15'])
            self.assertEqual(v['lifecycle'], 'running')
            # Kill the direct PTY wrapper through the owning server's known child
            # relationship; retain a live pidfd on Linux to exclude PID reuse.
            if not sys.platform.startswith('linux'):
                self.skipTest('Linux pidfd fixture; no foreign platform claim')
            record = json.loads((state(box) / 'registry.json').read_text())['agents'][0]
            sock = box.root / 'runtime/txm/v1' / ('s-' + record['binding']['native_instance'] + '.sock')
            with socket.socket(socket.AF_UNIX) as connection:
                connection.connect(str(sock)); server_pid = struct.unpack('3i', connection.getsockopt(socket.SOL_SOCKET, socket.SO_PEERCRED, 12))[0]
            children = set()
            for path in Path(f'/proc/{server_pid}/task').glob('*/children'):
                try: children.update(path.read_text().split())
                except FileNotFoundError: pass
            children = sorted(children)
            self.assertEqual(len(children), 1)
            wrapper = int(children[0]); handle = os.pidfd_open(wrapper)
            try:
                self.assertIn(f'PPid:\t{server_pid}\n', Path(f'/proc/{wrapper}/status').read_text())
                signal.pidfd_send_signal(handle, signal.SIGKILL)
            finally:
                os.close(handle)
            wait_for(lambda: not sock.exists())
            observed = status(box, v['id'])
            self.assertEqual(observed['lifecycle'], 'unknown'); self.assertIsNone(observed['receipt'])

    def test_registration_has_no_supervised_identity_or_exit(self):
        with fixture() as box, coordinator(box):
            with native_server(box, BINARY, name='existing'):
                v = envelope(box, ['agent', 'register', 'native/existing'])['data']['agent']
                self.assertFalse(v['supervised']); self.assertIsNone(v['managed_id'])
                self.assertIsNone(v['instance_id']); self.assertIsNone(v['receipt'])
                self.assertEqual(v['lifecycle'], 'unknown')
            observed = status(box, v['id']); self.assertEqual(observed['lifecycle'], 'unknown')
            self.assertIsNone(observed['receipt'])

    def test_failed_exec_and_single_use_manifest(self):
        with fixture() as box, coordinator(box):
            invalid = box.root / 'bad-exec'; invalid.write_text('#!/does-not-exist\nPRIVATE_SYNTHETIC\n'); invalid.chmod(0o700)
            reply = envelope(box, ['agent', 'start', 'failed', '--', str(invalid)], 7)
            v = reply['data']['agent']; self.assertTrue(reply['partial'])
            self.assertEqual(v['receipt']['outcome'], 'start_failed')
            self.assertIsNone(v['receipt']['exit_code']); self.assertIsNone(v['receipt']['signal'])
            self.assertIsNone(v['receipt']['started_at'])
            self.assertNotIn('PRIVATE_SYNTHETIC', json.dumps(reply))
            live = start(box, 'one-use', ['/bin/sleep', '15'])
            duplicate = box.run([BINARY, 'internal', 'supervised-run', live['instance_id']], check=False)
            self.assertEqual(duplicate.returncode, 5)
            self.assertFalse((state(box) / ('m-' + live['instance_id'] + '.json')).exists())
            self.assertEqual(status(box, live['id'])['lifecycle'], 'running')
            envelope(box, ['agent', 'start', 'one-use', '--', '/usr/bin/true'], 5)

    def test_interrupted_launch_intent_is_unknown_and_never_replayed(self):
        with fixture() as box:
            with coordinator(box): envelope(box, ['agent', 'list'])
            ident, instance = secrets.token_hex(16), secrets.token_hex(16)
            registry = state(box) / 'registry.json'; data = json.loads(registry.read_text())
            marker = box.root / 'must-not-execute'
            data['agents'].append({'id': ident, 'managed_id': ident, 'instance_id': instance,
                'locator': {'host_id': data['host_id'], 'backend': 'native', 'name': 'interrupted'},
                'binding': {'backend_id': '', 'native_instance': '', 'owner': ''},
                'supervised': True, 'created_at': '2026-10-30T00:00:00Z'})
            registry.write_text(json.dumps(data))
            manifest = state(box) / ('m-' + instance + '.json')
            manifest.write_text(json.dumps({'schema_version': 1, 'managed_id': ident, 'instance_id': instance,
                'backend': 'native', 'argv': ['/bin/sh', '-c', 'printf bad > ' + str(marker)],
                'cwd': str(box.root / 'repo'), 'env_names': [], 'receipt': 'r-' + instance + '.json'}))
            manifest.chmod(0o600)
            with coordinator(box):
                v = status(box, ident); self.assertEqual(v['lifecycle'], 'unknown')
                self.assertIsNone(v['receipt']); self.assertFalse(marker.exists()); self.assertFalse(manifest.exists())

    @unittest.skipUnless(shutil.which('tmux'), 'tmux unavailable')
    def test_tmux_exact_launch_rename_and_reused_name(self):
        with fixture() as box:
            box.start_tmux(shutil.which('tmux'))
            with coordinator(box):
                cwd = box.root / "cwd #{$HOME} ';"; cwd.mkdir()
                result = box.root / 'tmux-result'; release = box.root / 'release'
                script = box.root / 'tmux.py'
                script.write_text(f'from pathlib import Path\nimport os,time\nPath({str(result)!r}).write_text(os.getcwd())\nwhile not Path({str(release)!r}).exists():time.sleep(.02)\nraise SystemExit(42)\n')
                v = start(box, 'mux', [sys.executable, str(script)], backend='tmux', extra=['--cwd', str(cwd)])
                wait_for(result.exists); self.assertEqual(result.read_text(), str(cwd))
                envelope(box, ['--backend', 'native', 'agent', 'status', v['id']], 2)
                tmux(box, 'rename-session', '-t', '=mux', 'renamed')
                self.assertEqual(status(box, v['id'])['locator']['name'], 'renamed')
                tmux(box, 'new-session', '-d', '-s', 'mux', 'exec /bin/sleep 120')
                release.touch(); self.assertEqual(await_exit(box, v['id'])['receipt']['exit_code'], 42)
                self.assertIn('mux', tmux(box, 'list-sessions', '-F', '#{session_name}'))
                # Register a separate unmanaged mux; no retroactive receipt.
                u = envelope(box, ['agent', 'register', 'tmux/mux'])['data']['agent']
                tmux(box, 'kill-session', '-t', '=mux'); tmux(box, 'new-session', '-d', '-s', 'mux', 'exec /bin/sleep 120')
                self.assertIsNone(status(box, u['id'])['receipt']); box.verify_tmux()
                for code in (0, 42):
                    quick = start(box, 'short-' + str(code), ['/bin/sh', '-c', 'exit ' + str(code)], backend='tmux')
                    self.assertEqual(await_exit(box, quick['id'])['receipt']['exit_code'], code)
                killed = start(box, 'self-signal', [sys.executable, '-c', 'import os,signal;os.kill(os.getpid(),signal.SIGUSR1)'], backend='tmux')
                receipt = await_exit(box, killed['id'])['receipt']
                self.assertEqual(receipt['outcome'], 'signaled'); self.assertIsNone(receipt['exit_code'])

    @unittest.skipUnless(shutil.which('tmux'), 'tmux unavailable')
    def test_tmux_coordinator_loss_preserves_receipt_without_reexecution(self):
        with fixture() as box:
            box.start_tmux(shutil.which('tmux'))
            count, release = box.root / 'count', box.root / 'release'
            script = box.root / 'once.py'
            script.write_text(f'from pathlib import Path\nimport time\np=Path({str(count)!r});p.write_text(p.read_text()+"x" if p.exists() else "x")\nwhile not Path({str(release)!r}).exists():time.sleep(.02)\nraise SystemExit(42)\n')
            with coordinator(box) as service:
                view = start(box, 'survives', [sys.executable, str(script)], backend='tmux')
                self.assertEqual(view['lifecycle'], 'running'); wait_for(count.exists)
                os.killpg(service.pid, signal.SIGKILL); service.wait(timeout=5)
                self.assertIn('survives', tmux(box, 'list-sessions', '-F', '#{session_name}'))
                release.touch()
                wait_for(lambda: (state(box) / ('r-' + view['instance_id'] + '.json')).exists())
            with coordinator(box):
                self.assertEqual(status(box, view['id'])['receipt']['exit_code'], 42)
                self.assertEqual(count.read_text(), 'x'); box.verify_tmux()

    @unittest.skipUnless(shutil.which('tmux'), 'tmux unavailable')
    def test_launcher_path_with_spaces_quotes_and_format_characters(self):
        with fixture() as box:
            box.start_tmux(shutil.which('tmux'))
            executable = box.root / "txm #{$HOME} ';"
            shutil.copy2(BINARY, executable); executable.chmod(0o700)
            with coordinator(box, executable):
                script = box.root / 'args.py'; output = box.root / 'args.json'
                script.write_text(f'import sys,json\nfrom pathlib import Path\nPath({str(output)!r}).write_text(json.dumps(sys.argv[1:]))\n')
                argv = ['-arg', 'Enter', "$(touch nope); 'quoted'", 'λ']
                view = start(box, 'quoted', [sys.executable, str(script), *argv], backend='tmux')
                self.assertEqual(await_exit(box, view['id'])['receipt']['exit_code'], 0)
                self.assertEqual(json.loads(output.read_text()), argv)
                self.assertFalse((box.root / 'repo/nope').exists())

    def test_duplicate_concurrent_starts_and_single_writer(self):
        with fixture() as box, coordinator(box) as service:
            def request(_):
                return box.run([BINARY, '--json', 'agent', 'start', 'same', '--', '/bin/sleep', '20'], check=False, timeout=22)
            with ThreadPoolExecutor(max_workers=4) as pool:
                results = list(pool.map(request, range(4)))
            self.assertEqual(sorted(x.returncode for x in results), [0, 5, 5, 5])
            self.assertEqual(len(json.loads((state(box) / 'registry.json').read_text())['agents']), 1)
            out = box.run([BINARY, 'internal', 'coordinator'], check=False, timeout=3)
            self.assertEqual(out.returncode, 5); self.assertIsNone(service.poll())

    def test_corrupt_registry_and_receipts_are_not_repaired(self):
        with fixture() as box:
            with coordinator(box):
                v = start(box, 'done', ['/usr/bin/true']); await_exit(box, v['id'])
                receipt = state(box) / ('r-' + v['instance_id'] + '.json')
                original = receipt.read_bytes(); receipt.write_text('{"schema_version":99}')
                envelope(box, ['agent', 'status', v['id']], 1)
                self.assertEqual(receipt.read_text(), '{"schema_version":99}'); receipt.write_bytes(original)
            registry = state(box) / 'registry.json'; registry.write_text('torn registry')
            out = box.run([BINARY, 'internal', 'coordinator'], check=False, timeout=3)
            self.assertNotEqual(out.returncode, 0); self.assertEqual(registry.read_text(), 'torn registry')
            box.run([BINARY, 'list'])

    def test_framing_epoch_and_slow_clients_do_not_block_native(self):
        with fixture() as box, coordinator(box):
            self.assertIsNone(rpc(box, {'op': 'ping'}, fragmented=True)['error'])
            self.assertEqual(rpc(box, {'op': 'ping', 'epoch': '0' * 32})['error']['code'], 'unsupported_version')
            for body in [b'{"op":"ping","op":"start"}', b'{"OP":"ping"}', b'{"unknown":"PRIVATE_SYNTHETIC"}']:
                reply = rpc(box, body); self.assertEqual(reply['error']['code'], 'protocol_error')
                self.assertNotIn('PRIVATE_SYNTHETIC', json.dumps(reply))
            metadata = json.loads((runtime(box) / 'coordinator.json').read_text())
            path = runtime(box) / ('s-' + metadata['epoch'] + '.sock')
            peers = []
            try:
                for _ in range(16):
                    peer = socket.socket(socket.AF_UNIX); peer.settimeout(1); peer.connect(str(path)); peer.sendall(b'\0'); peers.append(peer)
                with native_server(box, BINARY):
                    self.assertIn('sample', box.run([BINARY, 'list'], timeout=3).stdout)
            finally:
                for peer in peers: peer.close()

    def test_cancelled_agent_read_preserves_exit_130(self):
        with fixture() as box, coordinator(box):
            with native_server(box, BINARY, name='registered'):
                envelope(box, ['agent', 'register', 'native/registered'])
                path = box.root / 'runtime/txm/v1' / ('s-' + secrets.token_hex(16) + '.sock')
                with socket.socket(socket.AF_UNIX) as listener:
                    listener.bind(str(path)); path.chmod(0o600); listener.listen(); listener.settimeout(5)
                    proc = box.start([BINARY, '--json', 'agent', 'list'], stdout=subprocess.PIPE, stderr=subprocess.PIPE)
                    connection, _ = listener.accept()
                    with connection:
                        proc.terminate(); stdout, stderr = proc.communicate(timeout=4)
                        self.assertEqual(proc.returncode, 130)
                        self.assertEqual(json.loads(stdout)['errors'][0]['code'], 'cancelled')
                        self.assertEqual(stderr, b'')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(); parser.add_argument('--binary', type=Path, required=True)
    args, remaining = parser.parse_known_args(); BINARY = args.binary.resolve(strict=True)
    unittest.main(argv=[sys.argv[0], *remaining])
