#!/usr/bin/env python3
"""Installed macOS Screen/Zellij smoke; all commands use guarded private sockets."""
import argparse
from contextlib import contextmanager
import fcntl
import json
import os
from pathlib import Path
import secrets
import shutil
import stat
import struct
import subprocess
import sys
import termios
import unittest

from sandbox import Sandbox, wait_for
from tty_state import attributes

BINARY = None
OBSERVATIONS = {}


@contextmanager
def backend(kind):
    box = Sandbox()
    for name in ('cache', 'data', 'sockets'):
        (box.root/name).mkdir(mode=0o700)
    box.env.update(XDG_CACHE_HOME=str(box.root/'cache'), XDG_DATA_HOME=str(box.root/'data'),
                   SYSTEM_SCREENRC='/dev/null', SCREENRC='/dev/null',
                   ZELLIJ_SOCKET_DIR=str(box.root/'sockets'))
    executable = Path(shutil.which(kind)).resolve(strict=True)
    config = box.root/'zellij.kdl'
    config.write_text('default_shell "/bin/sh"\nsession_serialization false\nshow_startup_tips false\nshow_release_notes false\n')
    prefix = [str(executable), '-c', '/dev/null'] if kind == 'screen' else [str(executable), '--config', str(config)]
    namespace = box.root/('screen' if kind == 'screen' else 'sockets')
    variable = 'SCREENDIR' if kind == 'screen' else 'ZELLIJ_SOCKET_DIR'
    box.executable(kind, f'''#!{sys.executable}
import os,stat,sys
from pathlib import Path
root=Path({str(box.root)!r});info=root.lstat()
assert stat.S_ISDIR(info.st_mode) and info.st_uid==os.geteuid() and info.st_mode&0o777==0o700
assert (info.st_dev,info.st_ino)=={box.identity!r} and (root/'.owner').read_text()=={box.token!r}
assert os.environ.get({variable!r})=={str(namespace)!r}
os.execv({str(executable)!r},{prefix!r}+sys.argv[1:])
''')

    def command(args, code=0):
        result = box.run([BINARY, '--backend', kind, *args], timeout=15, check=False)
        assert result.returncode == code, (kind, args, result.returncode, result.stdout, result.stderr)
        return result

    def sessions():
        return json.loads(command(['--json', 'list']).stdout)['data']['sessions']

    def socket_for(row):
        matches = [p for p in namespace.rglob('*') if p.name == row['backend_id']]
        assert len(matches) == 1, (kind, row, matches)
        path = matches[0]; info = path.lstat()
        expected_type = stat.S_ISFIFO(info.st_mode) if kind == 'screen' else stat.S_ISSOCK(info.st_mode)
        assert expected_type and info.st_uid == os.geteuid() and not path.is_symlink()
        assert path.resolve().is_relative_to(namespace.resolve())
        return path, (info.st_dev, info.st_ino)

    owned = {}
    intended = set()
    unconfirmed = set()
    children = []
    stop = box.root/'stop-children'
    wrapper = box.root/'child-wrapper.py'
    wrapper.write_text('''import json,signal,subprocess,sys,time
from pathlib import Path
manifest=Path(sys.argv[1]);value=json.loads(manifest.read_text());stopping=False
def stop(*_):
 global stopping
 stopping=True
signal.signal(signal.SIGHUP,stop);signal.signal(signal.SIGTERM,stop)
p=subprocess.Popen(value['argv'])
while p.poll() is None:
 if stopping or Path(value['stop']).exists():
  p.terminate()
  try:p.wait(timeout=.5)
  except subprocess.TimeoutExpired:p.kill();p.wait(timeout=2)
 time.sleep(.02)
path=Path(value['done']);tmp=path.with_suffix('.tmp');tmp.write_text(str(p.wait()));tmp.replace(path)
''')

    def create(name, argv=None):
        intended.add(name)
        unconfirmed.add(name)
        done = box.root/('child-done-'+name)
        manifest = box.root/('child-'+name+'.json')
        manifest.write_text(json.dumps({'argv': list(map(str, argv or ['/bin/sleep', '120'])), 'stop': str(stop), 'done': str(done)}))
        children.append(done)
        command(['create', name, sys.executable, str(wrapper), str(manifest)])
        found = []
        def visible():
            found[:] = [row for row in sessions() if row['locator']['name'] == name]
            return len(found) == 1
        wait_for(visible, timeout=8)
        row = found[0]; owned[row['backend_id']] = socket_for(row); unconfirmed.remove(name)
        return row

    try:
        # Before creating anything, prove Screen's advertised namespace too.
        if kind == 'screen':
            empty = box.run(['screen', '-ls'], check=False)
            assert str(namespace) in empty.stdout, empty.stdout
        assert sessions() == []
        yield box, command, sessions, create
    finally:
        # A backend may orphan its login/PTY child when the server exits. The
        # fixture wrapper is the sole reaper and stops its own live Popen child;
        # retain the namespace until all completion markers have arrived.
        stop.touch()
        wait_for(lambda: all(path.exists() for path in children), timeout=6)
        # Exact IDs and unchanged owned socket inodes authorize cleanup. No PID
        # discovery/signals, default sockets, kill-server or user configuration.
        for row in sessions():
            key = row['backend_id']
            if key not in owned:
                # A failed reply may precede discovery in this exclusive fixture
                # namespace. Require its launch intent and an owned entry before
                # obtaining the exact ID; this is not real task ownership proof.
                assert row['locator']['name'] in intended, ('unexpected fixture session; retained', box.root, row)
                owned[key] = socket_for(row)
                unconfirmed.discard(row['locator']['name'])
            path, identity = owned[key]; info = path.lstat()
            assert (info.st_dev, info.st_ino) == identity
            command(['delete', key])
        wait_for(lambda: not sessions(), timeout=8)
        assert not unconfirmed, ('unconfirmed launch intent; fixture retained', box.root, unconfirmed)
        OBSERVATIONS.setdefault(kind, {})['all_fixture_children_reaped'] = True
        box.close()


class MacOSBackendChecks(unittest.TestCase):
    def test_lifecycle_exact_targets_refusals_and_namespace_scoped_nuke(self):
        for kind in ('screen', 'zellij'):
            with self.subTest(backend=kind), backend(kind) as (_, _, guard_sessions, guard_create), backend(kind) as (box, cmd, sessions, create):
                guard = guard_create('protected_'+secrets.token_hex(4))
                long = create('foobar')
                cmd(['delete', 'foo'], 3)
                self.assertEqual(sessions()[0]['backend_id'], long['backend_id'])
                short = create('foo')
                for args in (['attach', '--read-only', 'foo'], ['exec', 'foo', '0', '0', 'FORBIDDEN'], ['session', 'output', 'foo']):
                    cmd(args, 4)
                cmd(['delete', short['backend_id']]); wait_for(lambda: len(sessions()) == 1)
                cmd(['nuke']); wait_for(lambda: not sessions())
                self.assertEqual(guard_sessions()[0]['backend_id'], guard['backend_id'])
                capability = json.loads(cmd(['--json', 'backend', 'capabilities']).stdout)['data']['backends'][0]
                self.assertTrue(capability['available'])
                self.assertEqual(capability['capabilities']['session_lifecycle']['level'], 'degraded')
                self.assertTrue(capability['version'])
                OBSERVATIONS.setdefault(kind, {}).update({'version': capability['version'],
                                      'exact_name_collision': True, 'private_socket_placement': True,
                                      'unsupported_refusals': True, 'protected_namespace_survived_nuke': True})

    def test_actual_attach_input_detach_and_tty_restoration(self):
        for kind in ('screen', 'zellij'):
            with self.subTest(backend=kind), backend(kind) as (box, _, sessions, create):
                marker = box.root/'received'; receiver = box.root/'receiver.py'
                receiver.write_text(f'import os\nfrom pathlib import Path\nwhile True:\n value=os.read(0,4096)\n if not value:break\n if b"MAC_BACKEND_INPUT" in value:Path({str(marker)!r}).touch()\n')
                row = create('attach_test', [sys.executable, str(receiver)])
                master, slave = os.openpty(); before = attributes(slave)
                os.set_blocking(master, False)
                def drain():
                    try: os.read(master, 65536)
                    except BlockingIOError: pass
                try:
                    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack('HHHH', 30, 120, 0, 0))
                    proc = box.start([BINARY, '--backend', kind, 'attach', row['backend_id']], stdin=slave, stdout=slave, stderr=slave)
                    wait_for(lambda: (drain(), not (attributes(slave)[3] & termios.ICANON))[1], timeout=8)
                    os.write(master, b'MAC_BACKEND_INPUT\n')
                    wait_for(lambda: (drain(), marker.exists())[1], timeout=8)
                    os.write(master, b'\x01d' if kind == 'screen' else b'\x0fd')
                    wait_for(lambda: (drain(), proc.poll() is not None)[1], timeout=8)
                    self.assertEqual(proc.returncode, 0)
                    self.assertEqual(attributes(slave), before)
                    self.assertEqual(sessions()[0]['backend_id'], row['backend_id'])
                    OBSERVATIONS.setdefault(kind, {})['attach_input_detach_tty'] = True
                finally:
                    os.close(master); os.close(slave)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True, type=Path)
    parser.add_argument('--report', required=True, type=Path)
    args, remaining = parser.parse_known_args()
    if sys.platform != 'darwin': parser.error('requires an actual macOS host')
    for name in ('screen', 'zellij'):
        if not shutil.which(name): parser.error(name+' is required; no skipped backend pass')
    BINARY = args.binary.resolve(strict=True)
    result = unittest.main(argv=[sys.argv[0], *remaining], exit=False).result
    args.report.write_text(json.dumps({'passed': result.wasSuccessful(), 'observations': OBSERVATIONS,
                                      'scope': 'macOS installed backends, isolated namespaces; capabilities remain degraded'}, indent=2)+'\n')
    raise SystemExit(0 if result.wasSuccessful() else 1)
